package main

import (
	"archive/zip"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

type Config struct {
	CIDRs       string `json:"cidrs"`
	Ports       string `json:"ports"`
	TimeoutMS   int    `json:"timeout_ms"`
	Concurrency int    `json:"concurrency"`
	Shuffle     bool   `json:"shuffle"`
	OnlyAlive   bool   `json:"only_alive"`
	CheckHTTP   bool   `json:"check_http"`
}

type Result struct {
	IP        string  `json:"ip"`
	Alive     bool    `json:"alive"`
	OpenPorts []int   `json:"open_ports"`
	LatencyMS float64 `json:"latency_ms"`
	HTTPCode  int     `json:"http_code"`
}

type ProbeConfig struct {
	UUID        string `json:"uuid"`
	ServerAddr  string `json:"server_addr"`
	ServerPort  int    `json:"server_port"`
	SNI         string `json:"sni"`
	WSPath      string `json:"ws_path"`
	TestURL     string `json:"test_url"`
	Concurrency int    `json:"concurrency"`
	TimeoutMS   int    `json:"timeout_ms"`
	AutoRanges  bool   `json:"auto_ranges"`
	CIDRs       string `json:"cidrs"`
}

type ProbeResult struct {
	IP        string  `json:"ip"`
	Success   bool    `json:"success"`
	LatencyMS int64   `json:"latency_ms"`
	HTTPCode  int     `json:"http_code"`
}

type StatsMsg struct {
	Total   int64   `json:"total"`
	Scanned int64   `json:"scanned"`
	Alive   int64   `json:"alive"`
	Rate    float64 `json:"rate"`
	Elapsed float64 `json:"elapsed"`
}

type Msg struct {
	Type string      `json:"type"`
	Data interface{} `json:"data"`
}

type Hub struct {
	mu      sync.RWMutex
	clients map[chan string]struct{}
}

func newHub() *Hub { return &Hub{clients: make(map[chan string]struct{})} }

func (h *Hub) add(ch chan string) {
	h.mu.Lock()
	h.clients[ch] = struct{}{}
	h.mu.Unlock()
}

func (h *Hub) remove(ch chan string) {
	h.mu.Lock()
	delete(h.clients, ch)
	h.mu.Unlock()
}

func (h *Hub) broadcast(m Msg) {
	b, _ := json.Marshal(m)
	line := "data: " + string(b) + "\n\n"
	h.mu.RLock()
	for ch := range h.clients {
		select {
		case ch <- line:
		default:
		}
	}
	h.mu.RUnlock()
}

var (
	hub      = newHub()
	isActive int32
	cancelMu sync.Mutex
	cancelFn context.CancelFunc
)

var cfRanges = []string{
	"103.21.244.0/22", "103.22.200.0/22", "103.31.4.0/22",
	"104.16.0.0/13", "104.24.0.0/14", "108.162.192.0/18",
	"131.0.72.0/22", "141.101.64.0/18", "162.158.0.0/15",
	"172.64.0.0/13", "173.245.48.0/20", "188.114.96.0/20",
	"190.93.240.0/20", "197.234.240.0/22", "198.41.128.0/17",
}

func generateIPs(cidrStr string) ([]string, error) {
	var all []string
	seen := map[string]bool{}
	for _, line := range strings.Split(cidrStr, "\n") {
		cidr := strings.TrimSpace(line)
		if cidr == "" || strings.HasPrefix(cidr, "#") {
			continue
		}
		if !strings.Contains(cidr, "/") {
			if ip := net.ParseIP(cidr); ip != nil {
				s := ip.String()
				if !seen[s] {
					seen[s] = true
					all = append(all, s)
				}
			}
			continue
		}
		_, network, err := net.ParseCIDR(cidr)
		if err != nil {
			return nil, fmt.Errorf("invalid CIDR %q: %v", cidr, err)
		}
		for ip := cloneIP(network.IP); network.Contains(ip); incIP(ip) {
			s := ip.String()
			if !seen[s] {
				seen[s] = true
				all = append(all, s)
			}
			if len(all) >= 2_000_000 {
				return all, nil
			}
		}
	}
	return all, nil
}

func cloneIP(ip net.IP) net.IP {
	cp := make(net.IP, len(ip))
	copy(cp, ip)
	return cp
}

func incIP(ip net.IP) {
	for i := len(ip) - 1; i >= 0; i-- {
		ip[i]++
		if ip[i] != 0 {
			break
		}
	}
}

func parsePorts(s string) []int {
	seen := map[int]bool{}
	var out []int
	for _, tok := range strings.Split(s, ",") {
		tok = strings.TrimSpace(tok)
		if tok == "" {
			continue
		}
		if strings.Contains(tok, "-") {
			parts := strings.SplitN(tok, "-", 2)
			a, ea := strconv.Atoi(strings.TrimSpace(parts[0]))
			b, eb := strconv.Atoi(strings.TrimSpace(parts[1]))
			if ea == nil && eb == nil && a > 0 && b <= 65535 && a <= b {
				for p := a; p <= b; p++ {
					if !seen[p] {
						seen[p] = true
						out = append(out, p)
					}
				}
			}
			continue
		}
		n, err := strconv.Atoi(tok)
		if err != nil || n < 1 || n > 65535 {
			continue
		}
		if !seen[n] {
			seen[n] = true
			out = append(out, n)
		}
	}
	sort.Ints(out)
	return out
}

func tcpDial(ctx context.Context, ip string, port int, timeout time.Duration) (float64, bool) {
	addr := fmt.Sprintf("%s:%d", ip, port)
	t0 := time.Now()
	conn, err := (&net.Dialer{Timeout: timeout}).DialContext(ctx, "tcp", addr)
	ms := float64(time.Since(t0).Microseconds()) / 1000.0
	if err != nil {
		return 0, false
	}
	conn.Close()
	return ms, true
}

func httpGet(ctx context.Context, ip string, port int, timeout time.Duration) int {
	scheme := "http"
	tlsPorts := map[int]bool{443: true, 8443: true, 2083: true, 2087: true, 2096: true}
	if tlsPorts[port] {
		scheme = "https"
	}
	url := fmt.Sprintf("%s://%s:%d/", scheme, ip, port)
	client := &http.Client{
		Timeout: timeout,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	req, err := http.NewRequestWithContext(ctx, "HEAD", url, nil)
	if err != nil {
		return 0
	}
	req.Header.Set("User-Agent", "curl/7.88.0")
	resp, err := client.Do(req)
	if err != nil {
		return 0
	}
	resp.Body.Close()
	return resp.StatusCode
}

func scanOne(ctx context.Context, ip string, ports []int, timeout time.Duration, checkHTTP bool) Result {
	res := Result{IP: ip}
	type pr struct {
		port int
		ms   float64
		ok   bool
	}
	prs := make([]pr, len(ports))
	var wg sync.WaitGroup
	for i, p := range ports {
		wg.Add(1)
		go func(idx, port int) {
			defer wg.Done()
			ms, ok := tcpDial(ctx, ip, port, timeout)
			prs[idx] = pr{port, ms, ok}
		}(i, p)
	}
	wg.Wait()
	best := 1e9
	for _, r := range prs {
		if r.ok {
			res.Alive = true
			res.OpenPorts = append(res.OpenPorts, r.port)
			if r.ms < best {
				best = r.ms
				res.LatencyMS = r.ms
			}
		}
	}
	if res.Alive && checkHTTP {
		httpPorts := map[int]bool{80: true, 443: true, 8080: true, 8443: true,
			2053: true, 2083: true, 2086: true, 2087: true, 2096: true}
		for _, p := range res.OpenPorts {
			if httpPorts[p] {
				code := httpGet(ctx, ip, p, timeout)
				if code > 0 {
					res.HTTPCode = code
					break
				}
			}
		}
	}
	return res
}

func doScan(ctx context.Context, ips []string, ports []int, cfg Config) {
	defer atomic.StoreInt32(&isActive, 0)
	total := int64(len(ips))
	var scanned, alive int64
	timeout := time.Duration(cfg.TimeoutMS) * time.Millisecond
	t0 := time.Now()
	sem := make(chan struct{}, cfg.Concurrency)
	var wg sync.WaitGroup
	statsDone := make(chan struct{})
	go func() {
		defer close(statsDone)
		tick := time.NewTicker(400 * time.Millisecond)
		defer tick.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-tick.C:
				s := atomic.LoadInt64(&scanned)
				a := atomic.LoadInt64(&alive)
				el := time.Since(t0).Seconds()
				rate := 0.0
				if el > 0 {
					rate = float64(s) / el
				}
				hub.broadcast(Msg{"stats", StatsMsg{total, s, a, rate, el}})
				if s >= total {
					return
				}
			}
		}
	}()
	for _, ip := range ips {
		select {
		case <-ctx.Done():
			goto done
		case sem <- struct{}{}:
		}
		wg.Add(1)
		go func(target string) {
			defer func() { <-sem; wg.Done() }()
			r := scanOne(ctx, target, ports, timeout, cfg.CheckHTTP)
			atomic.AddInt64(&scanned, 1)
			if r.Alive {
				atomic.AddInt64(&alive, 1)
				hub.broadcast(Msg{"result", r})
			} else if !cfg.OnlyAlive {
				hub.broadcast(Msg{"result", r})
			}
		}(ip)
	}
done:
	wg.Wait()
	<-statsDone
	el := time.Since(t0).Seconds()
	s := atomic.LoadInt64(&scanned)
	a := atomic.LoadInt64(&alive)
	hub.broadcast(Msg{"stats", StatsMsg{total, s, a, float64(s) / el, el}})
	hub.broadcast(Msg{"done", map[string]interface{}{"scanned": s, "alive": a, "elapsed": el}})
}

func ensureXray() (string, error) {
	candidates := []string{
		"./xray",
		"/tmp/t2hash-xray/xray",
		"/usr/local/bin/xray",
	}
	for _, p := range candidates {
		if info, err := os.Stat(p); err == nil && !info.IsDir() {
			return p, nil
		}
	}
	return downloadXray()
}

func downloadXray() (string, error) {
	arch := runtime.GOARCH
	var releaseArch string
	switch arch {
	case "amd64":
		releaseArch = "64"
	case "arm64":
		releaseArch = "arm64-v8a"
	default:
		return "", fmt.Errorf("unsupported arch: %s", arch)
	}

	hub.broadcast(Msg{"log", "دانلود xray-core..."})

	url := "https://github.com/XTLS/Xray-core/releases/latest/download/Xray-linux-" + releaseArch + ".zip"
	resp, err := http.Get(url)
	if err != nil {
		return "", fmt.Errorf("download failed: %v", err)
	}
	defer resp.Body.Close()

	tmp, err := os.CreateTemp("", "xray-*.zip")
	if err != nil {
		return "", err
	}
	defer os.Remove(tmp.Name())

	if _, err := io.Copy(tmp, resp.Body); err != nil {
		tmp.Close()
		return "", err
	}
	tmp.Close()

	destDir := "/tmp/t2hash-xray"
	os.MkdirAll(destDir, 0755)
	destPath := filepath.Join(destDir, "xray")

	zr, err := zip.OpenReader(tmp.Name())
	if err != nil {
		return "", err
	}
	defer zr.Close()

	for _, f := range zr.File {
		if f.Name == "xray" {
			src, err := f.Open()
			if err != nil {
				return "", err
			}
			dst, err := os.OpenFile(destPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0755)
			if err != nil {
				src.Close()
				return "", err
			}
			_, err = io.Copy(dst, src)
			src.Close()
			dst.Close()
			if err != nil {
				return "", err
			}
			hub.broadcast(Msg{"log", "xray دانلود شد: " + destPath})
			return destPath, nil
		}
	}
	return "", fmt.Errorf("xray binary not found in zip")
}

func buildXrayJSON(cdnIP string, localPort int, cfg ProbeConfig) string {
	sni := cfg.SNI
	if sni == "" {
		sni = cfg.ServerAddr
	}
	path := cfg.WSPath
	if path == "" {
		path = "/"
	}
	port := cfg.ServerPort
	if port == 0 {
		port = 443
	}
	return fmt.Sprintf(`{"log":{"loglevel":"none"},"inbounds":[{"listen":"127.0.0.1","port":%d,"protocol":"socks","settings":{"auth":"noauth","udp":false}}],"outbounds":[{"protocol":"vless","settings":{"vnext":[{"address":"%s","port":%d,"users":[{"id":"%s","encryption":"none","flow":""}]}]},"streamSettings":{"network":"ws","security":"tls","tlsSettings":{"serverName":"%s","allowInsecure":false,"fingerprint":"chrome"},"wsSettings":{"path":"%s","headers":{"Host":"%s"}}}}]}`,
		localPort, cdnIP, port, cfg.UUID, sni, path, sni)
}

func socks5Dial(socksAddr, targetHost string, targetPort int, timeout time.Duration) (net.Conn, error) {
	conn, err := net.DialTimeout("tcp", socksAddr, timeout)
	if err != nil {
		return nil, err
	}
	conn.SetDeadline(time.Now().Add(timeout))

	if _, err := conn.Write([]byte{5, 1, 0}); err != nil {
		conn.Close()
		return nil, err
	}
	buf := make([]byte, 2)
	if _, err := io.ReadFull(conn, buf); err != nil {
		conn.Close()
		return nil, err
	}
	if buf[1] != 0 {
		conn.Close()
		return nil, fmt.Errorf("socks5 auth rejected")
	}

	hostBytes := []byte(targetHost)
	req := []byte{5, 1, 0, 3, byte(len(hostBytes))}
	req = append(req, hostBytes...)
	req = append(req, byte(targetPort>>8), byte(targetPort&0xff))
	if _, err := conn.Write(req); err != nil {
		conn.Close()
		return nil, err
	}

	hdr := make([]byte, 4)
	if _, err := io.ReadFull(conn, hdr); err != nil {
		conn.Close()
		return nil, err
	}
	if hdr[1] != 0 {
		conn.Close()
		return nil, fmt.Errorf("socks5 connect failed: %d", hdr[1])
	}
	switch hdr[3] {
	case 1:
		io.ReadFull(conn, make([]byte, 6))
	case 3:
		lb := make([]byte, 1)
		io.ReadFull(conn, lb)
		io.ReadFull(conn, make([]byte, int(lb[0])+2))
	case 4:
		io.ReadFull(conn, make([]byte, 18))
	}

	conn.SetDeadline(time.Time{})
	return conn, nil
}

func testViaSocks(ctx context.Context, socksAddr, testURL string, timeout time.Duration) (int64, int, bool) {
	host := "cp.cloudflare.com"
	port := 80
	path := "/generate_204"

	if testURL != "" {
		if strings.HasPrefix(testURL, "http://") {
			rest := strings.TrimPrefix(testURL, "http://")
			parts := strings.SplitN(rest, "/", 2)
			host = parts[0]
			if len(parts) > 1 {
				path = "/" + parts[1]
			} else {
				path = "/"
			}
		} else if strings.HasPrefix(testURL, "https://") {
			rest := strings.TrimPrefix(testURL, "https://")
			parts := strings.SplitN(rest, "/", 2)
			host = parts[0]
			port = 443
			if len(parts) > 1 {
				path = "/" + parts[1]
			} else {
				path = "/"
			}
		}
	}

	t0 := time.Now()
	conn, err := socks5Dial(socksAddr, host, port, timeout)
	if err != nil {
		return 0, 0, false
	}
	defer conn.Close()

	conn.SetDeadline(time.Now().Add(timeout))
	req := fmt.Sprintf("GET %s HTTP/1.1\r\nHost: %s\r\nUser-Agent: curl/7.88.0\r\nConnection: close\r\n\r\n", path, host)
	if _, err := conn.Write([]byte(req)); err != nil {
		return 0, 0, false
	}

	buf := make([]byte, 512)
	n, _ := conn.Read(buf)
	latency := time.Since(t0).Milliseconds()

	if n < 12 {
		return latency, 0, false
	}
	statusLine := string(buf[:n])
	codeStr := ""
	if idx := strings.Index(statusLine, "HTTP/"); idx >= 0 {
		parts := strings.Fields(statusLine[idx:])
		if len(parts) >= 2 {
			codeStr = parts[1]
		}
	}
	code, _ := strconv.Atoi(codeStr)
	success := code >= 200 && code < 500
	return latency, code, success
}

func probeOneIP(ctx context.Context, cdnIP string, cfg ProbeConfig, xrayBin string) ProbeResult {
	localPort := 21000 + rand.Intn(20000)

	xrayCfgJSON := buildXrayJSON(cdnIP, localPort, cfg)

	tmp, err := os.CreateTemp("", "xray-cfg-*.json")
	if err != nil {
		return ProbeResult{IP: cdnIP}
	}
	tmp.WriteString(xrayCfgJSON)
	tmp.Close()
	defer os.Remove(tmp.Name())

	cmd := exec.CommandContext(ctx, xrayBin, "run", "-c", tmp.Name())
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	if err := cmd.Start(); err != nil {
		return ProbeResult{IP: cdnIP}
	}
	defer func() {
		cmd.Process.Kill()
		cmd.Wait()
	}()

	time.Sleep(350 * time.Millisecond)

	socksAddr := fmt.Sprintf("127.0.0.1:%d", localPort)
	timeout := time.Duration(cfg.TimeoutMS) * time.Millisecond
	if timeout < time.Second {
		timeout = 8 * time.Second
	}

	lat, code, ok := testViaSocks(ctx, socksAddr, cfg.TestURL, timeout)
	return ProbeResult{IP: cdnIP, Success: ok, LatencyMS: lat, HTTPCode: code}
}

func doProbe(ctx context.Context, ips []string, cfg ProbeConfig, xrayBin string) {
	defer atomic.StoreInt32(&isActive, 0)

	total := int64(len(ips))
	var scanned, alive int64
	t0 := time.Now()

	conc := cfg.Concurrency
	if conc < 1 || conc > 50 {
		conc = 10
	}

	sem := make(chan struct{}, conc)
	var wg sync.WaitGroup

	statsDone := make(chan struct{})
	go func() {
		defer close(statsDone)
		tick := time.NewTicker(800 * time.Millisecond)
		defer tick.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-tick.C:
				s := atomic.LoadInt64(&scanned)
				a := atomic.LoadInt64(&alive)
				el := time.Since(t0).Seconds()
				rate := 0.0
				if el > 0 {
					rate = float64(s) / el
				}
				hub.broadcast(Msg{"probe_stats", StatsMsg{total, s, a, rate, el}})
				if s >= total {
					return
				}
			}
		}
	}()

	for _, ip := range ips {
		select {
		case <-ctx.Done():
			goto done
		case sem <- struct{}{}:
		}
		wg.Add(1)
		go func(target string) {
			defer func() { <-sem; wg.Done() }()
			r := probeOneIP(ctx, target, cfg, xrayBin)
			atomic.AddInt64(&scanned, 1)
			if r.Success {
				atomic.AddInt64(&alive, 1)
			}
			hub.broadcast(Msg{"probe_result", r})
		}(ip)
	}

done:
	wg.Wait()
	<-statsDone
	el := time.Since(t0).Seconds()
	s := atomic.LoadInt64(&scanned)
	a := atomic.LoadInt64(&alive)
	hub.broadcast(Msg{"probe_done", map[string]interface{}{"scanned": s, "working": a, "elapsed": el}})
}

func cors(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET,POST,OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		if r.Method == http.MethodOptions {
			w.WriteHeader(204)
			return
		}
		next(w, r)
	}
}

func apiStart(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST only", 405)
		return
	}
	if !atomic.CompareAndSwapInt32(&isActive, 0, 1) {
		w.WriteHeader(409)
		json.NewEncoder(w).Encode(map[string]string{"error": "already running"})
		return
	}
	var cfg Config
	if err := json.NewDecoder(r.Body).Decode(&cfg); err != nil {
		atomic.StoreInt32(&isActive, 0)
		http.Error(w, err.Error(), 400)
		return
	}
	if cfg.TimeoutMS < 100 || cfg.TimeoutMS > 30000 {
		cfg.TimeoutMS = 2000
	}
	if cfg.Concurrency < 1 || cfg.Concurrency > 5000 {
		cfg.Concurrency = 500
	}
	if cfg.Ports == "" {
		cfg.Ports = "80,443,8080,8443,2053,2083,2086,2087,2096"
	}
	ports := parsePorts(cfg.Ports)
	if len(ports) == 0 {
		atomic.StoreInt32(&isActive, 0)
		http.Error(w, "no valid ports", 400)
		return
	}
	ips, err := generateIPs(cfg.CIDRs)
	if err != nil {
		atomic.StoreInt32(&isActive, 0)
		http.Error(w, err.Error(), 400)
		return
	}
	if len(ips) == 0 {
		atomic.StoreInt32(&isActive, 0)
		http.Error(w, "no IPs", 400)
		return
	}
	if cfg.Shuffle {
		rand.Shuffle(len(ips), func(i, j int) { ips[i], ips[j] = ips[j], ips[i] })
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancelMu.Lock()
	cancelFn = cancel
	cancelMu.Unlock()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"status": "started", "total": len(ips), "ports": ports})
	go doScan(ctx, ips, ports, cfg)
}

func apiProbeStart(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST only", 405)
		return
	}
	if !atomic.CompareAndSwapInt32(&isActive, 0, 1) {
		w.WriteHeader(409)
		json.NewEncoder(w).Encode(map[string]string{"error": "already running"})
		return
	}
	var cfg ProbeConfig
	if err := json.NewDecoder(r.Body).Decode(&cfg); err != nil {
		atomic.StoreInt32(&isActive, 0)
		http.Error(w, err.Error(), 400)
		return
	}
	if cfg.UUID == "" || cfg.ServerAddr == "" {
		atomic.StoreInt32(&isActive, 0)
		http.Error(w, "uuid and server_addr required", 400)
		return
	}
	if cfg.ServerPort == 0 {
		cfg.ServerPort = 443
	}
	if cfg.TimeoutMS < 1000 {
		cfg.TimeoutMS = 8000
	}
	if cfg.TestURL == "" {
		cfg.TestURL = "http://cp.cloudflare.com/generate_204"
	}

	cidrStr := cfg.CIDRs
	if cfg.AutoRanges || cidrStr == "" {
		cidrStr = strings.Join(cfRanges, "\n")
	}

	ips, err := generateIPs(cidrStr)
	if err != nil {
		atomic.StoreInt32(&isActive, 0)
		http.Error(w, err.Error(), 400)
		return
	}
	rand.Shuffle(len(ips), func(i, j int) { ips[i], ips[j] = ips[j], ips[i] })

	xrayBin, err := ensureXray()
	if err != nil {
		atomic.StoreInt32(&isActive, 0)
		http.Error(w, "xray not found: "+err.Error(), 500)
		return
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancelMu.Lock()
	cancelFn = cancel
	cancelMu.Unlock()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":   "started",
		"total":    len(ips),
		"xray_bin": xrayBin,
	})
	go doProbe(ctx, ips, cfg, xrayBin)
}

func apiStop(w http.ResponseWriter, r *http.Request) {
	cancelMu.Lock()
	if cancelFn != nil {
		cancelFn()
	}
	cancelMu.Unlock()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "stopping"})
}

func apiStatus(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]bool{"active": atomic.LoadInt32(&isActive) == 1})
}

func sseHandler(w http.ResponseWriter, r *http.Request) {
	f, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming not supported", 500)
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	ch := make(chan string, 512)
	hub.add(ch)
	defer hub.remove(ch)
	ping := time.NewTicker(20 * time.Second)
	defer ping.Stop()
	for {
		select {
		case <-r.Context().Done():
			return
		case <-ping.C:
			fmt.Fprint(w, ": ping\n\n")
			f.Flush()
		case msg, ok := <-ch:
			if !ok {
				return
			}
			fmt.Fprint(w, msg)
			f.Flush()
		}
	}
}


type ISPInfo struct {
	IP      string `json:"ip"`
	ISP     string `json:"isp"`
	AS      string `json:"as"`
	Org     string `json:"org"`
	City    string `json:"city"`
	Country string `json:"country"`
}

func apiISP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get("http://ip-api.com/json?fields=status,query,isp,as,org,city,country")
	if err != nil {
		json.NewEncoder(w).Encode(ISPInfo{ISP: "Unknown", AS: "—"})
		return
	}
	defer resp.Body.Close()
	var info struct {
		Query   string `json:"query"`
		ISP     string `json:"isp"`
		AS      string `json:"as"`
		Org     string `json:"org"`
		City    string `json:"city"`
		Country string `json:"country"`
	}
	json.NewDecoder(resp.Body).Decode(&info)
	json.NewEncoder(w).Encode(ISPInfo{
		IP:      info.Query,
		ISP:     info.ISP,
		AS:      info.AS,
		Org:     info.Org,
		City:    info.City,
		Country: info.Country,
	})
}

func main() {
	port := "8080"
	if len(os.Args) > 1 {
		port = os.Args[1]
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprint(w, frontendHTML)
	})
	mux.HandleFunc("/events", cors(sseHandler))
	mux.HandleFunc("/api/scan/start", cors(apiStart))
	mux.HandleFunc("/api/probe/start", cors(apiProbeStart))
	mux.HandleFunc("/api/scan/stop", cors(apiStop))
	mux.HandleFunc("/api/status", cors(apiStatus))
	mux.HandleFunc("/api/isp", cors(apiISP))
	fmt.Printf("\n  t2hash-scanner v1.0 → http://localhost:%s\n\n", port)
	log.Fatal(http.ListenAndServe(":"+port, mux))
}




const frontendHTML = `<!DOCTYPE html>
<html lang="fa">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<title>t2hash-scanner</title>
<style>
:root {
  --bg:    #03050b;
  --s1:    #070c18;
  --s2:    #0b1222;
  --s3:    #0f1830;
  --bdr:   #14253d;
  --bdr2:  #1e3555;
  --g1:    #00ff88;
  --g2:    #00c8ff;
  --g3:    #8b5cf6;
  --g4:    #f472b6;
  --warn:  #fbbf24;
  --red:   #f87171;
  --txt:   #b8d4f0;
  --dim:   #2d4a6a;
  --font:  'Courier New', Courier, monospace;
}
*,*::before,*::after{box-sizing:border-box;margin:0;padding:0;}
body{background:var(--bg);color:var(--txt);font-family:var(--font);font-size:12.5px;height:100vh;display:flex;flex-direction:column;overflow:hidden;}

canvas#bg{position:fixed;inset:0;pointer-events:none;z-index:0;}

/* ── HEADER ── */
.hdr{
  position:relative;z-index:20;
  background:rgba(7,12,24,0.95);
  border-bottom:1px solid var(--bdr);
  padding:8px 18px;display:flex;align-items:center;gap:14px;
  backdrop-filter:blur(16px);flex-shrink:0;
}
.hdr::after{
  content:'';position:absolute;bottom:-1px;left:0;right:0;height:1px;
  background:linear-gradient(90deg,transparent,var(--g2),var(--g3),transparent);
  opacity:0.6;
}
.rwr{position:relative;width:38px;height:38px;flex-shrink:0;}
.rwr svg{width:38px;height:38px;}
.rsw{transform-origin:19px 19px;opacity:0;transition:opacity .3s;}
.scanning .rsw{opacity:1;animation:sweep 2s linear infinite;}
@keyframes sweep{to{transform:rotate(360deg);}}
.rpng{position:absolute;inset:0;border-radius:50%;border:2px solid var(--g1);opacity:0;pointer-events:none;}
.rpng.go{animation:rp .7s ease-out forwards;}
@keyframes rp{0%{transform:scale(.1);opacity:1}100%{transform:scale(3);opacity:0}}
.logo{display:flex;flex-direction:column;gap:2px;}
.lg1{
  font-size:16px;font-weight:900;letter-spacing:4px;
  background:linear-gradient(90deg,var(--g1) 0%,var(--g2) 40%,var(--g3) 70%,var(--g4) 100%);
  -webkit-background-clip:text;-webkit-text-fill-color:transparent;background-clip:text;
  filter:drop-shadow(0 0 8px rgba(0,200,255,0.4));
}
.lg2{color:var(--dim);font-size:9px;letter-spacing:2px;}
.hsp{flex:1;}
.socials{display:flex;gap:6px;}
.sl{
  display:inline-flex;align-items:center;gap:4px;
  background:rgba(14,25,48,0.8);border:1px solid var(--bdr);
  border-radius:3px;padding:3px 9px;font-size:10px;color:var(--dim);
  text-decoration:none;letter-spacing:.5px;transition:all .15s;
  position:relative;overflow:hidden;
}
.sl::before{content:'';position:absolute;inset:0;background:linear-gradient(90deg,transparent,rgba(255,255,255,0.03),transparent);transform:translateX(-100%);transition:transform .3s;}
.sl:hover::before{transform:translateX(100%);}
.sl.yt:hover{border-color:#ff0000;color:#ff0000;box-shadow:0 0 10px rgba(255,0,0,0.2);}
.sl.tg:hover{border-color:#29a8eb;color:#29a8eb;box-shadow:0 0 10px rgba(41,168,235,0.2);}
.sl.gh:hover{border-color:var(--txt);color:var(--txt);box-shadow:0 0 10px rgba(184,212,240,0.1);}
.pill{background:rgba(11,18,34,0.8);border:1px solid var(--bdr);border-radius:20px;padding:4px 12px;font-size:10px;letter-spacing:2px;color:var(--dim);transition:all .3s;}
.pill.on{border-color:var(--g1);color:var(--g1);background:rgba(0,255,136,0.07);box-shadow:0 0 16px rgba(0,255,136,0.2),inset 0 0 16px rgba(0,255,136,0.05);}
.pill.probe{border-color:var(--g3);color:var(--g3);background:rgba(139,92,246,0.07);box-shadow:0 0 16px rgba(139,92,246,0.2);}

/* ── LAYOUT ── */
.body{display:flex;flex:1;overflow:hidden;position:relative;z-index:1;}

/* ── SIDEBAR ── */
.sidebar{
  width:295px;flex-shrink:0;
  background:rgba(7,12,24,0.7);
  border-right:1px solid var(--bdr);
  display:flex;flex-direction:column;
  overflow-y:auto;overflow-x:hidden;
  backdrop-filter:blur(12px);
}
::-webkit-scrollbar{width:3px;}
::-webkit-scrollbar-track{background:transparent;}
::-webkit-scrollbar-thumb{background:var(--bdr2);border-radius:2px;}

/* MODE TABS */
.mtabs{display:grid;grid-template-columns:1fr 1fr;border-bottom:1px solid var(--bdr);}
.mtab{
  padding:10px;font-family:var(--font);font-size:11px;
  letter-spacing:1.5px;cursor:pointer;background:transparent;
  border:none;border-bottom:2px solid transparent;
  color:var(--dim);transition:all .2s;
}
.mtab.on{color:var(--g2);border-bottom-color:var(--g2);background:rgba(0,200,255,0.04);}
.mtab:hover:not(.on){color:var(--txt);background:rgba(255,255,255,.02);}

/* PANELS */
.tp{display:none;flex-direction:column;}
.tp.on{display:flex;}

/* 3D SECTION CARDS */
.sc{
  padding:13px 14px;
  border-bottom:1px solid rgba(20,37,61,0.5);
  position:relative;
  transform:perspective(600px) rotateY(-1.5deg);
  transform-style:preserve-3d;
  transition:transform .3s ease, box-shadow .3s ease, background .3s;
  background:rgba(11,18,34,0.3);
}
.sc::before{
  content:'';position:absolute;inset:0;
  background:linear-gradient(135deg,rgba(0,200,255,0.03) 0%,transparent 60%,rgba(139,92,246,0.02) 100%);
  pointer-events:none;opacity:0;transition:opacity .3s;
}
.sc::after{
  content:'';position:absolute;left:0;top:0;bottom:0;width:2px;
  background:linear-gradient(180deg,transparent,var(--g2),transparent);
  opacity:0;transition:opacity .3s;
}
.sc:hover{
  transform:perspective(600px) rotateY(0deg) translateX(4px);
  background:rgba(11,18,34,0.6);
  box-shadow:-4px 0 24px rgba(0,200,255,0.08),4px 0 0 rgba(0,0,0,.4);
}
.sc:hover::before{opacity:1;}
.sc:hover::after{opacity:1;}

/* SECTION HEADERS */
.sh{display:flex;align-items:center;gap:7px;margin-bottom:10px;}
.sd{width:6px;height:6px;border-radius:50%;flex-shrink:0;}
.sd.c{background:var(--g2);box-shadow:0 0 6px var(--g2);}
.sd.p{background:var(--g3);box-shadow:0 0 6px var(--g3);}
.sd.w{background:var(--warn);box-shadow:0 0 6px var(--warn);}
.sd.r{background:var(--g1);box-shadow:0 0 6px var(--g1);}
.sl2{font-size:9.5px;letter-spacing:2.5px;text-transform:uppercase;}
.sl2.c{color:var(--g2);}
.sl2.p{color:var(--g3);}
.sl2.w{color:var(--warn);}
.sl2.r{color:var(--g1);}

/* INPUTS */
.fi{display:flex;flex-direction:column;gap:3px;margin-bottom:8px;}
.fi:last-child{margin-bottom:0;}
.fi>label{color:var(--dim);font-size:9px;letter-spacing:1.5px;text-transform:uppercase;}
textarea,.inp{
  background:rgba(3,5,11,0.8);
  border:1px solid var(--bdr);
  color:var(--txt);font-family:var(--font);font-size:12px;
  padding:6px 9px;border-radius:3px;outline:none;width:100%;
  transition:border-color .2s,box-shadow .2s,background .2s;
}
textarea{resize:vertical;min-height:60px;line-height:1.6;}
textarea:focus,.inp:focus{
  border-color:var(--g2);
  box-shadow:0 0 0 2px rgba(0,200,255,0.1),inset 0 0 12px rgba(0,200,255,0.03);
  background:rgba(3,5,11,0.95);
}
select.inp{cursor:pointer;}
.g2{display:grid;grid-template-columns:1fr 1fr;gap:8px;}
.g3{display:grid;grid-template-columns:1fr 1fr 1fr;gap:6px;}

/* TAGS */
.tags{display:flex;flex-wrap:wrap;gap:3px;}
.tag{
  background:rgba(20,37,61,0.5);border:1px solid var(--bdr);
  color:var(--dim);font-family:var(--font);font-size:10px;
  padding:2px 8px;cursor:pointer;border-radius:2px;
  transition:all .15s;position:relative;overflow:hidden;
}
.tag:hover{border-color:var(--g2);color:var(--g2);background:rgba(0,200,255,0.07);}

/* CHECKBOXES */
.chk{display:flex;align-items:center;gap:6px;color:var(--dim);font-size:11px;cursor:pointer;margin-bottom:5px;user-select:none;}
.chk:last-child{margin-bottom:0;}
.chk input{width:auto;accent-color:var(--g1);cursor:pointer;}

/* BUTTONS */
.btn{
  display:inline-flex;align-items:center;justify-content:center;gap:5px;
  border:1px solid;font-family:var(--font);cursor:pointer;
  border-radius:3px;letter-spacing:1.5px;font-weight:bold;
  transition:all .15s;position:relative;overflow:hidden;
}
.btn::after{content:'';position:absolute;inset:0;background:rgba(255,255,255,0);transition:background .15s;}
.btn:active::after{background:rgba(255,255,255,0.05);}
.btn:disabled{opacity:.25;cursor:not-allowed;}

.btn-go{padding:9px 0;font-size:12px;width:100%;border-color:var(--g1);color:var(--g1);background:rgba(0,255,136,0.06);}
.btn-go:not(:disabled):hover{background:rgba(0,255,136,0.16);box-shadow:0 0 20px rgba(0,255,136,0.2),inset 0 0 20px rgba(0,255,136,0.03);}

.btn-stop{padding:9px 0;font-size:12px;width:100%;border-color:var(--red);color:var(--red);background:rgba(248,113,113,0.06);}
.btn-stop:not(:disabled):hover{background:rgba(248,113,113,0.16);}

.btn-prb{padding:9px 0;font-size:12px;width:100%;border-color:var(--g3);color:var(--g3);background:rgba(139,92,246,0.06);}
.btn-prb:not(:disabled):hover{background:rgba(139,92,246,0.18);box-shadow:0 0 20px rgba(139,92,246,0.2);}

.brow{display:grid;grid-template-columns:1fr 1fr;gap:7px;margin-bottom:7px;}
.btn-sm{padding:5px 8px;font-size:10px;letter-spacing:.5px;border-color:var(--bdr);color:var(--dim);background:transparent;}
.btn-sm:hover:not(:disabled){border-color:var(--bdr2);color:var(--txt);}
.btn-dl{padding:5px 8px;font-size:10px;letter-spacing:.5px;border-color:var(--g3);color:var(--g3);background:rgba(139,92,246,0.06);}
.btn-dl:hover:not(:disabled){background:rgba(139,92,246,0.15);}
.btn-cp{padding:3px 7px;font-size:9.5px;letter-spacing:.3px;border-color:var(--g2);color:var(--g2);background:rgba(0,200,255,0.05);}
.btn-cp:hover{background:rgba(0,200,255,0.12);}

/* XRAY NOTE */
.xnote{
  font-size:10px;color:rgba(139,92,246,0.8);line-height:1.6;
  background:rgba(139,92,246,0.05);
  border:1px solid rgba(139,92,246,0.2);
  border-radius:3px;padding:7px 9px;
}

/* EXPORT PANEL */
.export-panel{display:none;}
.export-panel.show{display:block;}
.cfg-item{
  background:rgba(3,5,11,.8);border:1px solid var(--bdr);
  border-radius:3px;padding:7px 9px;margin-bottom:5px;
  position:relative;
}
.cfg-ip{color:var(--g2);font-size:11px;font-weight:bold;}
.cfg-lat{color:var(--warn);font-size:10px;margin-left:8px;}
.cfg-link{
  color:var(--dim);font-size:9px;word-break:break-all;
  font-family:var(--font);margin-top:4px;line-height:1.5;
}
.cfg-actions{display:flex;gap:5px;margin-top:5px;}

/* ── CONTENT ── */
.content{flex:1;display:flex;flex-direction:column;overflow:hidden;min-width:0;}

/* ISP BANNER */
.isp-banner{
  flex-shrink:0;
  background:rgba(7,12,24,0.9);
  border-bottom:1px solid var(--bdr);
  padding:7px 16px;
  display:flex;align-items:center;gap:16px;
  min-height:42px;
  backdrop-filter:blur(8px);
  position:relative;overflow:hidden;
}
.isp-banner::before{
  content:'';position:absolute;inset:0;
  background:linear-gradient(90deg,transparent,rgba(0,200,255,0.02),transparent);
  animation:shimmer 4s ease infinite;
}
@keyframes shimmer{0%{transform:translateX(-100%)}100%{transform:translateX(100%)}}
.isp-dot{width:8px;height:8px;border-radius:50%;flex-shrink:0;animation:blink2 2s ease infinite;}
@keyframes blink2{0%,100%{opacity:1}50%{opacity:.4}}
.isp-info{display:flex;align-items:center;gap:12px;flex:1;flex-wrap:wrap;}
.isp-badge{
  display:inline-flex;align-items:center;gap:5px;
  padding:3px 9px;border-radius:2px;border:1px solid;
  font-size:10px;letter-spacing:1px;font-weight:bold;
}
.isp-field{font-size:10px;color:var(--dim);}
.isp-field span{color:var(--txt);}
.isp-note{
  font-size:9.5px;color:rgba(100,160,200,0.5);
  padding:2px 8px;
  border-left:1px solid var(--bdr);
}

/* STATS GRID */
.sgrid{
  display:grid;grid-template-columns:repeat(6,1fr);
  background:rgba(7,12,24,0.85);
  border-bottom:1px solid var(--bdr);
  flex-shrink:0;
}
.sc2{
  padding:10px 14px;
  border-right:1px solid var(--bdr);
  display:flex;flex-direction:column;gap:2px;
  transform:perspective(300px) rotateX(-1deg);
  transition:transform .2s,background .2s;
  cursor:default;
}
.sc2:last-child{border-right:none;}
.sc2:hover{transform:perspective(300px) rotateX(0deg) translateY(-2px);background:rgba(0,200,255,0.04);}
.slb{color:var(--dim);font-size:9px;letter-spacing:1.5px;text-transform:uppercase;}
.sv{font-size:20px;font-weight:900;line-height:1;font-variant-numeric:tabular-nums;transition:all .3s;}
.sv.g{background:linear-gradient(90deg,var(--g1),var(--g2));-webkit-background-clip:text;-webkit-text-fill-color:transparent;background-clip:text;}
.sv.c{color:var(--g2);}
.sv.a{color:var(--g1);text-shadow:0 0 12px rgba(0,255,136,0.5);}
.sv.w{color:var(--warn);}
.sv.t{color:var(--txt);}
.sv.p{background:linear-gradient(90deg,var(--g2),var(--g3));-webkit-background-clip:text;-webkit-text-fill-color:transparent;background-clip:text;}

/* PROGRESS */
.ptrack{height:2px;background:var(--bdr);flex-shrink:0;position:relative;overflow:hidden;}
.pfill{
  height:100%;width:0%;
  background:linear-gradient(90deg,var(--g1),var(--g2));
  transition:width .35s ease;
  position:relative;
}
.pfill::after{
  content:'';position:absolute;right:0;top:0;bottom:0;width:20px;
  background:linear-gradient(90deg,transparent,rgba(255,255,255,0.4));
}
.pfill.p{background:linear-gradient(90deg,var(--g3),var(--g2));}

/* TABLE BAR */
.tbar{
  display:flex;align-items:center;gap:8px;
  padding:6px 14px;
  background:rgba(7,12,24,0.7);border-bottom:1px solid var(--bdr);
  flex-shrink:0;
}
.fi2{flex:1;max-width:230px;padding:3px 9px;font-size:11px;border-radius:3px;}
.ri{color:var(--dim);font-size:10px;flex:1;}
.mbdg{
  display:inline-flex;align-items:center;gap:4px;
  font-size:9px;letter-spacing:2px;padding:2px 8px;border-radius:2px;border:1px solid;
}
.mbdg.tcp{border-color:var(--g2);color:var(--g2);background:rgba(0,200,255,0.07);}
.mbdg.xray{border-color:var(--g3);color:var(--g3);background:rgba(139,92,246,0.07);}

/* TABLE */
.twrap{flex:1;overflow-y:auto;overflow-x:hidden;}
table{width:100%;border-collapse:collapse;}
thead th{
  position:sticky;top:0;z-index:5;
  background:#050b14;
  color:var(--dim);font-size:9.5px;letter-spacing:1.5px;text-transform:uppercase;
  padding:6px 12px;text-align:left;
  border-bottom:1px solid var(--bdr2);
  cursor:pointer;user-select:none;white-space:nowrap;
  transition:color .15s;
}
thead th:hover{color:var(--g2);}
tbody tr{border-bottom:1px solid rgba(20,37,61,0.35);transition:background .1s;}
tbody tr:hover{background:rgba(0,200,255,0.03);}
tbody tr.rn{animation:ri2 .2s ease;}
@keyframes ri2{from{opacity:0;transform:translateY(-4px)}to{opacity:1;transform:translateY(0)}}
tbody tr.ar{border-left:2px solid rgba(0,255,136,0.4);}
tbody tr.wr{border-left:2px solid rgba(139,92,246,0.5);}
td{padding:5px 12px;white-space:nowrap;vertical-align:middle;}
.tn{color:var(--dim);font-size:10px;width:40px;}
.tip{color:var(--g2);font-size:13px;font-weight:bold;letter-spacing:.5px;}
.lf{color:var(--g1)}.lm{color:var(--warn)}.ls{color:var(--red)}
.tp2{color:var(--g1);}
.h2{color:var(--g1)}.h3{color:var(--g2)}.h4{color:var(--warn)}.h5{color:var(--red)}
.sa{color:var(--g1);font-size:10.5px;letter-spacing:1px;}
.sd2{color:var(--dim);font-size:10.5px;}
.sw2{color:var(--g3);font-size:10.5px;letter-spacing:1px;}
.sf{color:var(--red);font-size:10.5px;}

/* COPY CELL */
.ccell{display:flex;align-items:center;gap:4px;}
.copy-mini{
  padding:1px 5px;font-size:8px;font-family:var(--font);
  border:1px solid var(--bdr);color:var(--dim);
  background:transparent;cursor:pointer;border-radius:2px;
  transition:all .15s;
}
.copy-mini:hover{border-color:var(--g2);color:var(--g2);}

/* EMPTY */
.empty{display:flex;flex-direction:column;align-items:center;justify-content:center;height:100%;gap:14px;color:var(--dim);}
.empty-txt{letter-spacing:4px;font-size:11px;}

/* FOOTER */
.footer{
  position:relative;z-index:20;
  background:rgba(7,12,24,0.9);border-top:1px solid var(--bdr);
  padding:4px 16px;display:flex;align-items:center;gap:14px;
  font-size:10px;color:var(--dim);flex-shrink:0;
}
.footer::before{content:'';position:absolute;top:-1px;left:0;right:0;height:1px;background:linear-gradient(90deg,transparent,var(--g3),var(--g2),transparent);opacity:0.3;}
#ll{flex:1;overflow:hidden;text-overflow:ellipsis;white-space:nowrap;}
#cst{transition:color .3s;}
#cst.ok{color:var(--g1);}
.fso{display:flex;gap:8px;}
.fsl{color:var(--dim);font-size:9.5px;text-decoration:none;transition:color .15s;}
.fsl:hover{color:var(--g2);}
.fsl.yt:hover{color:#ff0000;}
.fsl.tg:hover{color:#29a8eb;}

/* TOAST */
.toast{
  position:fixed;bottom:38px;right:18px;
  background:rgba(11,18,34,0.95);border:1px solid var(--bdr2);
  border-left:3px solid var(--g1);color:var(--txt);
  padding:9px 14px;border-radius:3px;font-size:11px;
  z-index:300;animation:ti .2s ease;max-width:320px;
  backdrop-filter:blur(8px);
}
.toast.e{border-left-color:var(--red);}
.toast.p{border-left-color:var(--g3);}
@keyframes ti{from{opacity:0;transform:translateY(8px)}to{opacity:1;transform:translateY(0)}}
</style>
</head>
<body>

<canvas id="bg"></canvas>

<!-- HEADER -->
<div class="hdr" id="hdr">
  <div class="rwr" id="rwr">
    <div class="rpng" id="rpng"></div>
    <svg viewBox="0 0 38 38" fill="none">
      <circle cx="19" cy="19" r="17" stroke="#14253d" stroke-width="1"/>
      <circle cx="19" cy="19" r="11" stroke="#14253d" stroke-width="1"/>
      <circle cx="19" cy="19" r="5"  stroke="#14253d" stroke-width="1"/>
      <line x1="19" y1="19" x2="19" y2="2"  stroke="#1e3555" stroke-width=".8"/>
      <line x1="2"  y1="19" x2="36" y2="19" stroke="#1e3555" stroke-width=".8"/>
      <line x1="19" y1="19" x2="36" y2="2"  stroke="#1e3555" stroke-width=".5"/>
      <line x1="19" y1="19" x2="2"  y2="36" stroke="#1e3555" stroke-width=".5"/>
      <g class="rsw" id="sw">
        <path d="M19 19 L19 2 A17 17 0 0 1 36 19 Z" fill="url(#rg)" opacity=".6"/>
        <line x1="19" y1="19" x2="19" y2="2" stroke="#00ff88" stroke-width="1.5"/>
      </g>
      <defs>
        <radialGradient id="rg" cx="50%" cy="50%" r="50%">
          <stop offset="0%" stop-color="#00ff88" stop-opacity=".6"/>
          <stop offset="100%" stop-color="#00ff88" stop-opacity="0"/>
        </radialGradient>
      </defs>
      <circle cx="19" cy="19" r="2" fill="#00ff88" filter="url(#glow)"/>
      <defs>
        <filter id="glow"><feGaussianBlur stdDeviation="1.5" result="blur"/><feMerge><feMergeNode in="blur"/><feMergeNode in="SourceGraphic"/></feMerge></filter>
      </defs>
    </svg>
  </div>
  <div class="logo">
    <div class="lg1">T2HASH-SCANNER</div>
    <div class="lg2">IP SCANNER + XRAY CDN PROBE  ///  v1.0</div>
  </div>
  <div class="hsp"></div>
  <div class="socials">
    <a class="sl yt" href="https://youtube.com/@T2hsh" target="_blank">▶ @T2hsh</a>
    <a class="sl tg" href="https://t.me/T2HASHCHANNEL" target="_blank">✈ @T2HASHCHANNEL</a>
    <a class="sl gh" href="https://github.com/T2HASH" target="_blank">⌥ @T2HASH</a>
  </div>
  <div class="pill" id="pill">IDLE</div>
</div>

<div class="body">

<!-- SIDEBAR -->
<div class="sidebar">
  <div class="mtabs">
    <button class="mtab on" id="tb0" onclick="sw('tcp')">⚡ TCP SCAN</button>
    <button class="mtab"    id="tb1" onclick="sw('xray')">⬡ XRAY PROBE</button>
  </div>

  <!-- TCP TAB -->
  <div class="tp on" id="tp0">

    <div class="sc">
      <div class="sh"><div class="sd c"></div><div class="sl2 c">Target</div></div>
      <div class="fi">
        <label>CIDR / IP — یک خط یک رنج</label>
        <textarea id="cidrs" rows="3" placeholder="104.16.0.0/13&#10;172.64.0.0/13&#10;8.8.8.8"></textarea>
      </div>
      <div class="tags" style="margin-bottom:6px">
        <button class="tag" onclick="addP('cf1')">CF /13</button>
        <button class="tag" onclick="addP('cf2')">CF /14</button>
        <button class="tag" onclick="addP('cfall')">CF All</button>
        <button class="tag" onclick="addP('gcore')">Gcore</button>
        <button class="tag" onclick="addP('fastly')">Fastly</button>
        <button class="tag" onclick="addP('arvan')">ArvanCloud</button>
        <button class="tag" onclick="setCIDR('')">✕</button>
      </div>
    </div>

    <div class="sc">
      <div class="sh"><div class="sd p"></div><div class="sl2 p">Ports</div></div>
      <div class="fi">
        <label>لیست پورت (range: 8000-8010)</label>
        <input class="inp" type="text" id="ports" value="80,443,8080,8443,2053,2083,2086,2087,2096">
      </div>
      <div class="tags">
        <button class="tag" onclick="setP('web')">Web</button>
        <button class="tag" onclick="setP('xr')">Xray/CF</button>
        <button class="tag" onclick="setP('warp')">WARP</button>
        <button class="tag" onclick="setP('ext')">Extended</button>
        <button class="tag" onclick="setP('common')">Common</button>
        <button class="tag" onclick="setP('vpn')">VPN Ports</button>
      </div>
    </div>

    <div class="sc">
      <div class="sh"><div class="sd w"></div><div class="sl2 w">Config</div></div>
      <div class="g2">
        <div class="fi"><label>Concurrency</label><input class="inp" type="number" id="conc" value="500" min="1" max="5000"></div>
        <div class="fi"><label>Timeout ms</label><input class="inp" type="number" id="tmo" value="2000" min="100" max="30000"></div>
      </div>
      <label class="chk"><input type="checkbox" id="ck-sh" checked> Shuffle IPs</label>
      <label class="chk"><input type="checkbox" id="ck-al" checked> فقط alive نشون بده</label>
      <label class="chk"><input type="checkbox" id="ck-ht"> بررسی HTTP response code</label>
      <label class="chk"><input type="checkbox" id="ck-sc" checked> Auto-scroll</label>
    </div>

    <div class="sc">
      <div class="brow">
        <button class="btn btn-go"   id="b0" onclick="startScan()">▶ START</button>
        <button class="btn btn-stop" id="b1" onclick="doStop()" disabled>■ STOP</button>
      </div>
      <div class="brow">
        <button class="btn btn-sm" onclick="clearAll()">⌫ CLEAR</button>
        <button class="btn btn-sm" onclick="filterAlive()">☑ Alive only</button>
      </div>
      <div class="brow" style="margin-top:7px;margin-bottom:0">
        <button class="btn btn-dl" onclick="dlCSV()">⬇ CSV</button>
        <button class="btn btn-dl" onclick="dlJSON()">⬇ JSON</button>
      </div>
    </div>

  </div>

  <!-- XRAY TAB -->
  <div class="tp" id="tp1">

    <div class="sc">
      <div class="sh"><div class="sd p"></div><div class="sl2 p">Xray Server</div></div>
      <div class="xnote">کانفیگ VLESS اتومات ساخته می‌شه با IP پیداشده به‌عنوان CDN — تست می‌کنه ترافیک واقعی رد می‌شه یا فقط ping</div>
    </div>

    <div class="sc">
      <div class="sh"><div class="sd r"></div><div class="sl2 r">Credentials</div></div>
      <div class="fi"><label>UUID</label><input class="inp" type="text" id="xu" placeholder="xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx"></div>
      <div class="fi"><label>Server / SNI</label><input class="inp" type="text" id="xs" placeholder="s1.example.com"></div>
      <div class="g2">
        <div class="fi"><label>Port</label><input class="inp" type="number" id="xp" value="443"></div>
        <div class="fi"><label>WS Path</label><input class="inp" type="text" id="xw" value="/vless"></div>
      </div>
    </div>

    <div class="sc">
      <div class="sh"><div class="sd c"></div><div class="sl2 c">Advanced Options</div></div>
      <div class="g2">
        <div class="fi">
          <label>Network</label>
          <select class="inp" id="xnet">
            <option value="ws" selected>WebSocket</option>
            <option value="grpc">gRPC</option>
            <option value="tcp">TCP</option>
          </select>
        </div>
        <div class="fi">
          <label>Security</label>
          <select class="inp" id="xsec">
            <option value="tls" selected>TLS</option>
            <option value="reality">Reality</option>
            <option value="none">None</option>
          </select>
        </div>
      </div>
      <div class="g2">
        <div class="fi">
          <label>Fingerprint</label>
          <select class="inp" id="xfp">
            <option value="chrome" selected>Chrome</option>
            <option value="firefox">Firefox</option>
            <option value="safari">Safari</option>
            <option value="randomized">Random</option>
          </select>
        </div>
        <div class="fi">
          <label>Flow</label>
          <select class="inp" id="xfl">
            <option value="" selected>None</option>
            <option value="xtls-rprx-vision">xtls-rprx-vision</option>
          </select>
        </div>
      </div>
      <div class="fi">
        <label>Test URL</label>
        <input class="inp" type="text" id="xurl" value="http://cp.cloudflare.com/generate_204">
      </div>
      <label class="chk"><input type="checkbox" id="xai"> Allow Insecure TLS</label>
      <label class="chk"><input type="checkbox" id="xsp"> Speed test on working IPs</label>
    </div>

    <div class="sc">
      <div class="sh"><div class="sd w"></div><div class="sl2 w">IP Range</div></div>
      <label class="chk"><input type="checkbox" id="xau" checked onchange="togXR()"> همه رنج‌های Cloudflare (auto)</label>
      <div id="xcw" style="display:none;margin-top:6px">
        <textarea class="inp" id="xcidrs" rows="3" placeholder="104.16.0.0/13&#10;172.64.0.0/13" style="font-size:11.5px"></textarea>
      </div>
      <div class="g2" style="margin-top:8px">
        <div class="fi"><label>Concurrency</label><input class="inp" type="number" id="xc" value="10" min="1" max="50"></div>
        <div class="fi"><label>Timeout ms</label><input class="inp" type="number" id="xt" value="8000" min="2000" max="30000"></div>
      </div>
    </div>

    <div class="sc">
      <button class="btn btn-prb" id="b2" onclick="startProbe()" style="margin-bottom:7px">⬡ START PROBE</button>
      <div class="brow">
        <button class="btn btn-stop" id="b3" onclick="doStop()" disabled>■ STOP</button>
        <button class="btn btn-sm"   onclick="clearAll()">⌫ CLEAR</button>
      </div>
      <div class="brow" style="margin-top:7px;margin-bottom:0">
        <button class="btn btn-dl" onclick="dlCSV()">⬇ CSV</button>
        <button class="btn btn-dl" onclick="dlJSON()">⬇ JSON</button>
      </div>
    </div>

    <div class="sc" id="ex-panel">
      <div class="sh"><div class="sd r"></div><div class="sl2 r">Working Configs</div></div>
      <div id="ex-list" style="max-height:200px;overflow-y:auto;"></div>
      <div id="ex-empty" style="color:var(--dim);font-size:10px;padding:4px 0">هنوز IP کاری پیدا نشده</div>
    </div>

  </div>
</div>

<!-- CONTENT -->
<div class="content">

  <!-- ISP BANNER -->
  <div class="isp-banner" id="isp-ban">
    <div class="isp-dot" id="isp-dot" style="background:var(--dim)"></div>
    <div class="isp-info" id="isp-info">
      <span style="color:var(--dim);font-size:10px;letter-spacing:1px">در حال تشخیص ISP سرور...</span>
    </div>
  </div>

  <!-- STATS -->
  <div class="sgrid">
    <div class="sc2"><div class="slb">Total IPs</div><div class="sv t" id="s0">—</div></div>
    <div class="sc2"><div class="slb">Scanned</div><div class="sv g" id="s1">0</div></div>
    <div class="sc2"><div class="slb" id="slb2">Alive</div><div class="sv a" id="s2">0</div></div>
    <div class="sc2"><div class="slb">Speed</div><div class="sv w" id="s3">0/s</div></div>
    <div class="sc2"><div class="slb">Elapsed</div><div class="sv t" id="s4">0s</div></div>
    <div class="sc2"><div class="slb">Progress</div><div class="sv p" id="s5">0%</div></div>
  </div>
  <div class="ptrack"><div class="pfill" id="pf"></div></div>

  <!-- TABLE BAR -->
  <div class="tbar">
    <input class="inp fi2" id="filt" type="text" placeholder="فیلتر IP…" oninput="apF()">
    <div class="ri" id="ri">0 نتیجه</div>
    <span class="mbdg tcp" id="mbdg">TCP</span>
    <button class="btn btn-sm" onclick="srtBy('lat')" style="padding:3px 9px;font-size:9px">↕ Lat</button>
    <button class="btn btn-sm" onclick="srtBy('ip')"  style="padding:3px 9px;font-size:9px">↕ IP</button>
    <button class="btn btn-sm" onclick="srtBy('status')" style="padding:3px 9px;font-size:9px">↕ St</button>
  </div>

  <!-- TABLE -->
  <div class="twrap" id="tw">
    <div class="empty" id="em">
      <svg width="90" height="90" viewBox="0 0 90 90" fill="none" opacity=".07">
        <circle cx="45" cy="45" r="42" stroke="#00c8ff" stroke-width="1.5"/>
        <circle cx="45" cy="45" r="28" stroke="#00c8ff" stroke-width="1"/>
        <circle cx="45" cy="45" r="14" stroke="#00c8ff" stroke-width="1"/>
        <line x1="45" y1="3" x2="45" y2="87" stroke="#00c8ff" stroke-width=".5"/>
        <line x1="3" y1="45" x2="87" y2="45" stroke="#00c8ff" stroke-width=".5"/>
        <circle cx="45" cy="45" r="3" fill="#00c8ff"/>
      </svg>
      <div class="empty-txt">AWAITING TARGET</div>
    </div>
    <table id="tbl" style="display:none">
      <thead><tr>
        <th class="tn">#</th>
        <th onclick="srtBy('ip')">IP ADDRESS</th>
        <th onclick="srtBy('lat')">LATENCY</th>
        <th id="ch3">OPEN PORTS</th>
        <th>HTTP</th>
        <th onclick="srtBy('status')">STATUS</th>
        <th id="ch5">—</th>
      </tr></thead>
      <tbody id="tb"></tbody>
    </table>
  </div>
</div>

</div><!-- body -->

<div class="footer">
  <span id="ll">آماده.</span>
  <div class="fso">
    <a class="fsl yt" href="https://youtube.com/@T2hsh" target="_blank">YT: @T2hsh</a>
    <span style="color:var(--bdr)">|</span>
    <a class="fsl tg" href="https://t.me/T2HASHCHANNEL" target="_blank">TG: @T2HASHCHANNEL</a>
    <span style="color:var(--bdr)">|</span>
    <a class="fsl" href="https://github.com/T2HASH" target="_blank">GH: @T2HASH</a>
  </div>
  <span id="cst">● disconnected</span>
</div>

<script>
/* ── 3D CANVAS BACKGROUND ── */
(function(){
  const c=document.getElementById('bg'),ctx=c.getContext('2d');
  let pts=[],lines=[];
  function rsz(){c.width=innerWidth;c.height=innerHeight;}
  rsz(); window.addEventListener('resize',rsz);
  for(let i=0;i<220;i++) pts.push({x:(Math.random()-0.5)*2000,y:(Math.random()-0.5)*1200,z:Math.random()*1400+50,vz:.5+Math.random()});
  lines=[
    [[0,-600],[0,600]],[[600,0],[-600,0]],
    [[-400,-600],[-400,600]],[[400,-600],[400,600]],
    [[-200,-600],[-200,600]],[[200,-600],[200,600]],
    [[-600,-300],[600,300]],[[-600,300],[600,-300]],
  ];
  function frame(){
    ctx.clearRect(0,0,c.width,c.height);
    const cx=c.width/2,cy=c.height/2;
    for(const p of pts){
      p.z-=p.vz; if(p.z<=1) p.z=1400;
      const s=700/p.z,px=p.x*s+cx,py=p.y*s+cy;
      if(px<-5||px>c.width+5||py<-5||py>c.height+5) continue;
      const r=Math.max(0.2,s*1.4);
      const a=Math.min(0.7,(1400-p.z)/1400*0.9);
      ctx.beginPath();ctx.arc(px,py,r,0,6.28);
      ctx.fillStyle='rgba(0,180,255,'+a+')';ctx.fill();
    }
    for(const [[x1,y1],[x2,y2]] of lines){
      const d1=600,d2=1400;
      const s1=700/d1,s2=700/d2;
      const gx1=x1*s1+cx,gy1=y1*s1+cy,gx2=x2*s2+cx,gy2=y2*s2+cy;
      const gr=ctx.createLinearGradient(gx1,gy1,gx2,gy2);
      gr.addColorStop(0,'rgba(0,200,255,0)');
      gr.addColorStop(.4,'rgba(0,200,255,0.04)');
      gr.addColorStop(.6,'rgba(139,92,246,0.03)');
      gr.addColorStop(1,'rgba(0,200,255,0)');
      ctx.beginPath();ctx.moveTo(gx1,gy1);ctx.lineTo(gx2,gy2);
      ctx.strokeStyle=gr;ctx.lineWidth=0.5;ctx.stroke();
    }
    requestAnimationFrame(frame);
  }
  frame();
})();

/* ── ISP DETECTION ── */
const ISP_COLORS = {
  'irancell':   {c:'#e91e8c',b:'rgba(233,30,140,0.1)',n:'Irancell / MCI2'},
  'rightel':    {c:'#e91e8c',b:'rgba(233,30,140,0.1)',n:'Rightel'},
  'hamrah':     {c:'#2196f3',b:'rgba(33,150,243,0.1)',n:'Hamrahe Aval / MCI'},
  'mci':        {c:'#2196f3',b:'rgba(33,150,243,0.1)',n:'Hamrahe Aval / MCI'},
  'mobile':     {c:'#2196f3',b:'rgba(33,150,243,0.1)',n:'MCI'},
  'mokhaberat': {c:'#ff9800',b:'rgba(255,152,0,0.1)',n:'Mokhaberat (TCI)'},
  'telecommunication':{c:'#ff9800',b:'rgba(255,152,0,0.1)',n:'TCI Mokhaberat'},
  'tci ':       {c:'#ff9800',b:'rgba(255,152,0,0.1)',n:'TCI'},
  'shatel':     {c:'#4caf50',b:'rgba(76,175,80,0.1)',n:'Shatel'},
  'asiatech':   {c:'#00bcd4',b:'rgba(0,188,212,0.1)',n:'Asiatech'},
  'pars':       {c:'#9c27b0',b:'rgba(156,39,176,0.1)',n:'Pars Online'},
  'hetzner':    {c:'#e53935',b:'rgba(229,57,53,0.1)',n:'Hetzner DE'},
  'digitalocean':{c:'#0066ff',b:'rgba(0,102,255,0.1)',n:'DigitalOcean'},
  'linode':     {c:'#00c08b',b:'rgba(0,192,139,0.1)',n:'Linode/Akamai'},
  'vultr':      {c:'#007bfc',b:'rgba(0,123,252,0.1)',n:'Vultr'},
};

function getISPColor(isp){
  const lo = (isp||'').toLowerCase();
  for(const [k,v] of Object.entries(ISP_COLORS)){
    if(lo.includes(k)) return v;
  }
  return {c:'#607d8b',b:'rgba(96,125,139,0.1)',n:isp};
}

async function loadISP(){
  try{
    const r = await fetch('/api/isp');
    const d = await r.json();
    if(!d.ip) return;
    const col = getISPColor(d.isp);
    const ban = document.getElementById('isp-ban');
    ban.style.borderBottom = '1px solid '+col.c+'40';
    ban.style.background   = col.b;
    document.getElementById('isp-dot').style.background  = col.c;
    document.getElementById('isp-dot').style.boxShadow   = '0 0 8px '+col.c;
    document.getElementById('isp-info').innerHTML =
      '<span class="isp-badge" style="border-color:'+col.c+';color:'+col.c+';background:'+col.b+'">'+
        '<span>●</span> '+(col.n||d.isp)+
      '</span>'+
      '<span class="isp-field">IP: <span>'+d.ip+'</span></span>'+
      '<span class="isp-field">'+d.as+'</span>'+
      '<span class="isp-field">'+d.city+', '+d.country+'</span>'+
      '<span class="isp-note">نتایج از دیدگاه '+(col.n||d.isp)+' هست</span>';
    log('ISP تشخیص داده شد: '+(col.n||d.isp)+' — '+d.ip);
  }catch(e){
    document.getElementById('isp-info').innerHTML='<span style="color:var(--dim);font-size:10px">ISP تشخیص داده نشد</span>';
  }
}
loadISP();

/* ── PRESETS ── */
const CIDRS={
  cf1:'104.16.0.0/13',cf2:'104.24.0.0/14',
  cfall:'103.21.244.0/22\n103.22.200.0/22\n103.31.4.0/22\n104.16.0.0/13\n104.24.0.0/14\n108.162.192.0/18\n131.0.72.0/22\n141.101.64.0/18\n162.158.0.0/15\n172.64.0.0/13\n173.245.48.0/20\n188.114.96.0/20\n190.93.240.0/20\n197.234.240.0/22\n198.41.128.0/17',
  gcore:'199.250.200.0/21\n199.34.228.0/22',
  fastly:'23.235.32.0/20\n43.249.72.0/22\n103.244.50.0/24\n151.101.0.0/17',
  arvan:'185.143.232.0/22\n92.114.16.80/29',
};
const PORTS={
  web:'80,443,8080,8443',
  xr:'80,443,2053,2083,2086,2087,2096,8080,8443,8880',
  warp:'500,854,859,864,878,880,890,891,894,2408,7103,7152,7156,8319,8742,8854,8886',
  ext:'80,443,2052,2053,2082,2083,2086,2087,2095,2096,8080,8443,8880,8888',
  common:'21,22,25,53,80,110,143,443,445,3306,3389,5432,6379,8080,8443,27017',
  vpn:'1194,1701,1723,500,4500,51820,8388,1080,3128',
};
function addP(k){const t=document.getElementById('cidrs');const v=CIDRS[k]||'';t.value=t.value.trim()?t.value.trim()+'\n'+v:v;}
function setCIDR(v){document.getElementById('cidrs').value=v;}
function setP(k){document.getElementById('ports').value=PORTS[k]||'';}
function togXR(){const a=document.getElementById('xau').checked;document.getElementById('xcw').style.display=a?'none':'block';}
function sw(t){
  document.getElementById('tp0').classList.toggle('on',t==='tcp');
  document.getElementById('tp1').classList.toggle('on',t==='xray');
  document.getElementById('tb0').classList.toggle('on',t==='tcp');
  document.getElementById('tb1').classList.toggle('on',t==='xray');
  const b=document.getElementById('mbdg');
  if(t==='tcp'){b.textContent='TCP';b.className='mbdg tcp';}
  else{b.textContent='XRAY';b.className='mbdg xray';}
  document.getElementById('ch3').textContent=t==='tcp'?'OPEN PORTS':'PROTO';
  document.getElementById('ch5').textContent=t==='tcp'?'—':'CONFIG';
  document.getElementById('slb2').textContent=t==='tcp'?'Alive':'Working';
}

/* ── STATE ── */
let allR=[],pending=[],flT=null,dRows=0,tRows=0,es=null;
const MAX=6000,FLUSHT=80;

/* ── SCAN ── */
async function startScan(){
  const cidrs=document.getElementById('cidrs').value.trim();
  const ports=document.getElementById('ports').value.trim();
  if(!cidrs){toast('یه CIDR وارد کن','e');return;}
  if(!ports){toast('حداقل یه پورت','e');return;}
  const cfg={cidrs,ports,
    timeout_ms:+document.getElementById('tmo').value||2000,
    concurrency:+document.getElementById('conc').value||500,
    shuffle:document.getElementById('ck-sh').checked,
    only_alive:document.getElementById('ck-al').checked,
    check_http:document.getElementById('ck-ht').checked,
  };
  try{
    const r=await fetch('/api/scan/start',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify(cfg)});
    if(!r.ok){toast(await r.text(),'e');return;}
    const d=await r.json();
    setT(d.total);setActive(true);connectSSE();
    toast('TCP اسکن: '+fmt(d.total)+' IP | ports: '+(d.ports||[]).slice(0,5).join(',')+' ...');
  }catch(e){toast(e.message,'e');}
}

async function startProbe(){
  const uuid=document.getElementById('xu').value.trim();
  const sni=document.getElementById('xs').value.trim();
  if(!uuid){toast('UUID رو وارد کن','e');return;}
  if(!sni){toast('آدرس سرور رو وارد کن','e');return;}
  const cfg={
    uuid,server_addr:sni,server_port:+document.getElementById('xp').value||443,
    sni,ws_path:document.getElementById('xw').value.trim()||'/vless',
    test_url:document.getElementById('xurl').value.trim()||'http://cp.cloudflare.com/generate_204',
    concurrency:+document.getElementById('xc').value||10,
    timeout_ms:+document.getElementById('xt').value||8000,
    auto_ranges:document.getElementById('xau').checked,
    cidrs:document.getElementById('xcidrs').value.trim(),
  };
  clearExports();
  try{
    const r=await fetch('/api/probe/start',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify(cfg)});
    if(!r.ok){toast(await r.text(),'e');return;}
    const d=await r.json();
    setT(d.total);setActive(true,true);connectSSE();
    toast('Xray probe: '+fmt(d.total)+' IP | bin: '+d.xray_bin,'p');
  }catch(e){toast(e.message,'e');}
}

async function doStop(){
  await fetch('/api/scan/stop',{method:'POST'}).catch(()=>{});
  toast('در حال توقف…');
}

function setActive(on,prb){
  ['b0','b2'].forEach(id=>{const el=document.getElementById(id);if(el)el.disabled=on;});
  ['b1','b3'].forEach(id=>{const el=document.getElementById(id);if(el)el.disabled=!on;});
  const p=document.getElementById('pill');
  if(!on){p.textContent='IDLE';p.className='pill';}
  else if(prb){p.textContent='PROBING';p.className='pill probe';}
  else{p.textContent='SCANNING';p.className='pill on';}
  document.getElementById('rwr').className=on?'rwr scanning':'rwr';
  if(prb) document.getElementById('pf').classList.add('p');
  else document.getElementById('pf').classList.remove('p');
}
function setT(n){document.getElementById('s0').textContent=fmt(n);}

/* ── SSE ── */
function connectSSE(){
  if(es)es.close();
  es=new EventSource('/events');
  es.onopen=()=>{const c=document.getElementById('cst');c.textContent='● connected';c.className='ok';};
  es.onerror=()=>{const c=document.getElementById('cst');c.textContent='● disconnected';c.className='';};
  es.onmessage=e=>{
    try{
      const m=JSON.parse(e.data);
      if(m.type==='result') onTcp(m.data);
      else if(m.type==='probe_result') onProbe(m.data);
      else if(m.type==='stats'||m.type==='probe_stats') onStats(m.data);
      else if(m.type==='done') onDone(m.data,false);
      else if(m.type==='probe_done') onDone(m.data,true);
      else if(m.type==='log') log(m.data);
    }catch(_){}
  };
}
connectSSE();

function onTcp(r){
  allR.push({...r,_t:'tcp'});
  const oa=document.getElementById('ck-al').checked;
  if(r.alive||!oa){pending.push({...r,_t:'tcp'});sch();}
  if(r.alive)ping();
}
function onProbe(r){
  allR.push({...r,_t:'prb'});
  pending.push({...r,_t:'prb'});
  sch();
  if(r.success){ping();addExport(r);}
}
function sch(){if(!flT)flT=setTimeout(flush,FLUSHT);}
function flush(){
  flT=null;
  if(!pending.length)return;
  const batch=pending.splice(0);
  const tb=document.getElementById('tb');
  const fr=document.createDocumentFragment();
  for(const r of batch){tRows++;dRows++;fr.appendChild(r._t==='prb'?bldPrb(r,tRows):bldTcp(r,tRows));}
  tb.appendChild(fr);
  while(tb.children.length>MAX){tb.removeChild(tb.firstChild);dRows--;}
  document.getElementById('tbl').style.display='';
  document.getElementById('em').style.display='none';
  updRI();
  if(document.getElementById('ck-sc').checked){const w=document.getElementById('tw');w.scrollTop=w.scrollHeight;}
  if(pending.length)sch();
}
function bldTcp(r,n){
  const tr=document.createElement('tr');
  tr.className=r.alive?'rn ar':'rn';
  tr.dataset.ip=r.ip;tr.dataset.alive=r.alive?1:0;
  tr.dataset.lat=r.alive?r.latency_ms:99999;
  const lat=r.alive?fmtL(r.latency_ms):'<span style="color:var(--dim)">—</span>';
  const pts=(r.open_ports||[]).join(', ')||'—';
  const ht=r.http_code?fmtH(r.http_code):'—';
  const st=r.alive?'<span class="sa">⬤ ALIVE</span>':'<span class="sd2">○ dead</span>';
  tr.innerHTML='<td class="tn">'+n+'</td><td class="tip">'+r.ip+'</td><td>'+lat+'</td><td class="tp2">'+pts+'</td><td>'+ht+'</td><td>'+st+'</td><td></td>';
  return tr;
}
function bldPrb(r,n){
  const tr=document.createElement('tr');
  tr.className=r.success?'rn wr':'rn';
  tr.dataset.ip=r.ip;tr.dataset.alive=r.success?1:0;tr.dataset.lat=r.latency_ms||99999;
  const lat=r.latency_ms?fmtL(r.latency_ms):'<span style="color:var(--dim)">—</span>';
  const ht=r.http_code?fmtH(r.http_code):'—';
  const st=r.success?'<span class="sw2">⬡ WORKING</span>':'<span class="sf">✕ failed</span>';
  const cp=r.success?'<button class="copy-mini" onclick="cpCfg(\''+r.ip+'\')">copy cfg</button>':'';
  tr.innerHTML='<td class="tn">'+n+'</td><td class="tip">'+r.ip+'</td><td>'+lat+'</td><td style="color:var(--dim);font-size:10px">VLESS+WS</td><td>'+ht+'</td><td>'+st+'</td><td>'+cp+'</td>';
  return tr;
}
function fmtL(ms){const c=ms<150?'lf':ms<500?'lm':'ls';return '<span class="'+c+'">'+ms.toFixed(1)+' ms</span>';}
function fmtH(c){const cl=c>=500?'h5':c>=400?'h4':c>=300?'h3':'h2';return '<span class="'+cl+'">'+c+'</span>';}

function onStats(s){
  document.getElementById('s1').textContent=fmt(s.scanned);
  document.getElementById('s2').textContent=fmt(s.alive);
  document.getElementById('s3').textContent=Math.round(s.rate)+'/s';
  document.getElementById('s4').textContent=s.elapsed.toFixed(1)+'s';
  if(s.total>0){const p=Math.round(s.scanned/s.total*100);document.getElementById('s5').textContent=p+'%';document.getElementById('pf').style.width=p+'%';document.getElementById('s0').textContent=fmt(s.total);}
}
function onDone(d,prb){
  setActive(false);
  const k=prb?'working':'alive';
  toast('✔ تمام — '+fmt(d.scanned)+' بررسی، '+fmt(d[k]||0)+' '+(prb?'کار می‌کنن':'alive')+' در '+d.elapsed.toFixed(1)+'ث');
  document.getElementById('pf').style.width='100%';
  setTimeout(()=>{document.getElementById('pf').style.width='0%';},4500);
}

/* ── EXPORT ── */
let workingIPs=[];
function addExport(r){
  workingIPs.push(r);
  document.getElementById('ex-empty').style.display='none';
  const uuid=document.getElementById('xu').value.trim();
  const sni=document.getElementById('xs').value.trim();
  const port=document.getElementById('xp').value||'443';
  const path=encodeURIComponent(document.getElementById('xw').value.trim()||'/vless');
  const fp=document.getElementById('xfp').value||'chrome';
  const link='vless://'+uuid+'@'+r.ip+':'+port+'?encryption=none&security=tls&sni='+sni+'&fp='+fp+'&type=ws&host='+sni+'&path='+path+'#t2hash-'+r.ip;
  const div=document.createElement('div');
  div.className='cfg-item';
  div.innerHTML='<div style="display:flex;align-items:center">'+'<span class="cfg-ip">'+r.ip+'</span>'+'<span class="cfg-lat">'+r.latency_ms+' ms</span>'+'</div>'+'<div class="cfg-link" id="cl-'+tRows+'">'+link+'</div>'+'<div class="cfg-actions"><button class="btn btn-cp" onclick="cp(\'cl-'+tRows+'\')">⎘ Copy Link</button></div>';
  document.getElementById('ex-list').appendChild(div);
}
function clearExports(){workingIPs=[];document.getElementById('ex-list').innerHTML='';document.getElementById('ex-empty').style.display='';}
function cp(id){const t=document.getElementById(id);if(!t)return;navigator.clipboard.writeText(t.textContent).then(()=>toast('لینک کپی شد'));} 
function cpCfg(ip){
  const uuid=document.getElementById('xu').value.trim();
  const sni=document.getElementById('xs').value.trim();
  const port=document.getElementById('xp').value||'443';
  const path=encodeURIComponent(document.getElementById('xw').value.trim()||'/vless');
  const fp=document.getElementById('xfp').value||'chrome';
  const link='vless://'+uuid+'@'+ip+':'+port+'?encryption=none&security=tls&sni='+sni+'&fp='+fp+'&type=ws&host='+sni+'&path='+path+'#t2hash-'+ip;
  navigator.clipboard.writeText(link).then(()=>toast('کانفیگ '+ip+' کپی شد'));
}

function clearAll(){
  allR=[];pending=[];tRows=0;dRows=0;workingIPs=[];
  if(flT){clearTimeout(flT);flT=null;}
  document.getElementById('tb').innerHTML='';
  document.getElementById('tbl').style.display='none';
  document.getElementById('em').style.display='';
  ['s1','s2','s3','s4'].forEach(id=>document.getElementById(id).textContent=id==='s3'?'0/s':'0');
  document.getElementById('s5').textContent='0%';document.getElementById('s0').textContent='—';
  document.getElementById('pf').style.width='0%';
  clearExports();updRI();log('پاک شد.');
}
function filterAlive(){document.querySelectorAll('#tb tr').forEach(t=>t.style.display=t.dataset.alive==='1'?'':'none');updRI();}
function apF(){const f=document.getElementById('filt').value.trim().toLowerCase();document.querySelectorAll('#tb tr').forEach(t=>{t.style.display=(!f||t.dataset.ip.includes(f))?'':'none';});updRI();}
function srtBy(col){
  const tb=document.getElementById('tb');
  const rows=Array.from(tb.querySelectorAll('tr'));
  rows.sort((a,b)=>{
    if(col==='ip') return a.dataset.ip.localeCompare(b.dataset.ip);
    if(col==='lat') return +a.dataset.lat - +b.dataset.lat;
    if(col==='status') return +b.dataset.alive - +a.dataset.alive;
    return 0;
  });
  const f=document.createDocumentFragment();
  rows.forEach((r,i)=>{r.cells[0].textContent=i+1;f.appendChild(r);});
  tb.appendChild(f);
}
function dlCSV(){
  if(!allR.length){toast('نتیجه‌ای نیست','e');return;}
  const isTcp=allR[0]._t==='tcp';
  let rows;
  if(isTcp) rows=[['IP','Alive','Latency(ms)','Open Ports','HTTP']].concat(allR.map(r=>[r.ip,r.alive?'true':'false',r.alive?r.latency_ms.toFixed(2):'',(r.open_ports||[]).join(';'),r.http_code||'']));
  else rows=[['IP','Working','Latency(ms)','HTTP Code']].concat(allR.map(r=>[r.ip,r.success?'true':'false',r.latency_ms||'',r.http_code||'']));
  dl(rows.map(r=>r.map(v=>'"'+String(v).replace(/"/g,'""')+'"').join(',')).join('\n'),'text/csv','t2hash-'+Date.now()+'.csv');
  toast('CSV — '+allR.length+' ردیف');
}
function dlJSON(){
  if(!allR.length){toast('نتیجه‌ای نیست','e');return;}
  dl(JSON.stringify({meta:{total:allR.length,exported:new Date().toISOString()},results:allR,working_vless_links:workingIPs.map(r=>{
    const uuid=document.getElementById('xu').value.trim(),sni=document.getElementById('xs').value.trim(),port=document.getElementById('xp').value||'443',path=encodeURIComponent(document.getElementById('xw').value.trim()||'/vless'),fp=document.getElementById('xfp').value||'chrome';
    return'vless://'+uuid+'@'+r.ip+':'+port+'?encryption=none&security=tls&sni='+sni+'&fp='+fp+'&type=ws&host='+sni+'&path='+path+'#t2hash-'+r.ip;
  })},null,2),'application/json','t2hash-'+Date.now()+'.json');
  toast('JSON دانلود شد');
}
function dl(cnt,tp,nm){const a=document.createElement('a');a.href=URL.createObjectURL(new Blob([cnt],{type:tp}));a.download=nm;a.click();setTimeout(()=>URL.revokeObjectURL(a.href),1000);}
function ping(){const p=document.getElementById('rpng');p.className='rpng';void p.offsetWidth;p.className='rpng go';}
function updRI(){const v=document.querySelectorAll('#tb tr:not([style*="none"])').length;document.getElementById('ri').textContent=fmt(v)+' نتیجه'+(allR.length>v?' (از '+fmt(allR.length)+')':'');}
function fmt(n){return Number(n).toLocaleString();}
function log(m){document.getElementById('ll').textContent=m;}
function toast(msg,t){log(msg);const d=document.createElement('div');d.className='toast'+(t?' '+t:'');d.textContent=msg;document.body.appendChild(d);setTimeout(()=>d.remove(),3500);}
</script>
</body>
</html>`
