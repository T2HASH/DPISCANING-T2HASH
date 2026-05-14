#!/bin/bash

# ==============================================================================
# Project: DPISCANING (V1.2 - ULTIMATE HYBRID)
# Creator: t2hash
# YouTube: @T2HSH | Telegram: t.me/t2hashchannel | GitHub: T2HASH
# ==============================================================================

set +m 2>/dev/null
shopt -s huponexit 2>/dev/null
exec 2>/dev/null 

R='\033[1;31m'; G='\033[1;32m'; Y='\033[1;33m'; B='\033[1;34m'
M='\033[1;35m'; C='\033[1;36m'; W='\033[1;37m'; NC='\033[0m'

check_deps() {
    for pkg in tcpdump curl bc; do
        if ! command -v $pkg &> /dev/null; then
            sudo apt-get install $pkg -y &>/dev/null
        fi
    done
}

show_header() {
    clear
    echo -e "${C}╔══════════════════════════════════════════════════════════════╗${NC}"
    echo -e "${C}║${NC} ${W}        S P E C T R E   E N G I N E   -   V 1 . 2          ${NC} ${C}║${NC}"
    echo -e "${C}║${NC} ${Y}              [ Layer-0 Hybrid Analysis ]               ${NC} ${C}║${NC}"
    echo -e "${C}╠══════════════════════════════════════════════════════════════╣${NC}"
    echo -e "${C}║${NC} ${G}Creator:${NC}  t2hash         ${G}YouTube:${NC}  @T2HSH                ${C}║${NC}"
    echo -e "${C}║${NC} ${G}GitHub:${NC}   T2HASH         ${G}Telegram:${NC} t.me/t2hashchannel    ${C}║${NC}"
    echo -e "${C}╚══════════════════════════════════════════════════════════════╝${NC}"
    echo ""
}

# --- Module 1: Live Packet Analyzer ---
module_1() {
    show_header
    echo -e "${W}--- [ 1. Live Packet Analyzer ] ---${NC}"
    read -p "Port ra vared konid (e.g. 443): " tp
    tp=${tp:-443}
    echo -e "\n${G}[!] Monitoring port $tp ... (Ctrl+C baraye khorooj)${NC}"
    sudo tcpdump -i any port $tp -nn -vv -X
}

# --- Module 2: Domain/SNI Tracker ---
module_2() {
    show_header
    echo -e "${W}--- [ 2. Domain / SNI Tracker ] ---${NC}"
    read -p "Domain SNI ra vared konid: " dom
    echo -e "\n${G}[!] Dar hal jostojoo baraye SNI: $dom ...${NC}"
    sudo tcpdump -i any port 443 -nn -A 2>/dev/null | grep --line-buffered -i "$dom"
}

# --- Module 3: Tunnel Health Analyzer ---
module_3() {
    show_header
    echo -e "${W}--- [ 3. Automated Status Analyzer ] ---${NC}"
    read -p "Domain (e.g. site.com): " dom
    echo -e "\n${C}[!] Dar hal tahlil-e vaziyat-e tunnel (15s)...${NC}"
    TMP=$(mktemp)
    sudo timeout 15 tcpdump -i any port 443 -nn -A -c 10 2>/dev/null | grep -i "$dom" -B 5 > "$TMP"
    if [ ! -s "$TMP" ]; then
        echo -e "${R}[✘] STATUS: Hich packeti daryaft nashod.${NC}"
    else
        echo -e "${G}[✔] STATUS: Packet ha ba movafaghiyat residand.${NC}"
        grep -q "Flags \[R\]" "$TMP" && echo -e "${R}[!] DETECTION: Reset Flag (Filtering).${NC}"
        grep -q "Flags \[P\.\]" "$TMP" && echo -e "${G}[✔] DETECTION: Data Flow OK (Push Flag).${NC}"
    fi
    rm -f "$TMP"; read -p "Enter baraye bazgasht..."
}

# --- Module 4: Cloudflare Deep Sweeper ---
module_4() {
    show_header
    echo -e "${W}--- [ 4. Smart Cloudflare Sweeper ] ---${NC}"
    read -p "Base Range (e.g. 188.114): " bip
    read -p "Block Start (0-255): " sb
    tput civis; TMP=$(mktemp)
    echo -e "${C}[!] Dar hal eskan range $bip Ba sorat-e Parallel...${NC}"
    for d in {1..100}; do
        ( t=$(curl -o /dev/null -s -w "%{time_connect}\n" --connect-timeout 1.2 http://$bip.$sb.$d)
          if [ "$t" != "0.000000" ] && [ -n "$t" ]; then echo "$t $bip.$sb.$d" >> "$TMP"; fi ) &
    done
    wait; tput cnorm
    echo -e "\n${Y}Top Clean IPs:${NC}"
    sort -n "$TMP" | head -n 5 | awk '{print " [✔] "$2" | Latency: "$1"s"}'
    rm -f "$TMP"; read -p "Enter baraye bazgasht..."
}

# --- Module 5: Arvan & DC Identity Sniper ---
module_5() {
    show_header
    echo -e "${W}--- [ 5. Network Identity & DC Sniper ] ---${NC}"
    p_ip=$(curl -s --connect-timeout 2 http://169.254.169.254/latest/meta-data/local-ipv4)
    pub_ip=$(curl -s --connect-timeout 3 https://api.ipify.org)
    echo -e "${G}[✔] Public IP: ${W}$pub_ip${NC}"
    [ -n "$p_ip" ] && echo -e "${G}[✔] Private IP (DC): ${W}$p_ip${NC}" || echo -e "${R}[✘] Metadata Blocked.${NC}"
    echo -e "\n${C}[!] Scanning Infrastructure Ports...${NC}"
    for p in 80 443 8080 2053; do
        (timeout 0.3 bash -c "cat < /dev/null > /dev/tcp/127.0.0.1/$p" 2>/dev/null) && \
        echo -e "${G} [✔] Port $p: OPEN${NC}" || echo -e "${R} [✘] Port $p: CLOSED${NC}"
    done
    read -p "Enter baraye bazgasht..."
}

# --- Module 6: DPI Flag Sniper ---
module_6() {
    show_header
    echo -e "${W}--- [ 6. Deep DPI Flag Analyzer ] ---${NC}"
    read -p "Port Target (443): " tp; tp=${tp:-443}
    echo -e "${C}[!] Analysing Packet Flags (10s)...${NC}"
    LOG=$(sudo timeout 10 tcpdump -i any port $tp -nn -vv -c 20 2>/dev/null)
    echo -e "\n${W}--- [ Result ] ---${NC}"
    if echo "$LOG" | grep -q "Flags \[R\]"; then
        echo -e "${R}[!!!] DETECTED: TCP Reset (DPI Blocking).${NC}"
    elif echo "$LOG" | grep -q "Flags \[P\.\]"; then
        echo -e "${G}[✔] DETECTED: Data Flowing (Push Flag).${NC}"
    else
        echo -e "${Y}[?] DETECTED: No Valid Flags / Timeout.${NC}"
    fi
    read -p "Enter baraye bazgasht..."
}

# --- Module 7: Triple-Hop Latency Grid ---
module_7() {
    show_header
    echo -e "${W}--- [ 7. Triple-Hop Latency Grid ] ---${NC}"
    read -p "Global Target: " target
    t1=$(curl -o /dev/null -s -w "%{time_connect}\n" http://127.0.0.1)
    t2=$(curl -o /dev/null -s -w "%{time_total}\n" --connect-timeout 4 https://$target)
    echo -e "${W}1. Local to Edge: ${G}${t1}s${NC}"
    echo -e "${W}2. Edge to Global: ${G}${t2}s${NC}"
    read -p "Enter baraye bazgasht..."
}

# --- Module 8: MTU Discovery (Bonus) ---
module_8() {
    show_header
    echo -e "${W}--- [ 8. MTU Path Discovery ] ---${NC}"
    read -p "Target IP: " tip
    echo -e "${C}[!] Finding Max Fragment Size...${NC}"
    for s in 1500 1450 1400 1350 1300; do
        if ping -c 1 -s $s -M do $tip &>/dev/null; then
            echo -e "${G}[✔] MTU $s: WORKS${NC}"; break
        else
            echo -e "${R}[✘] MTU $s: FAILED${NC}"
        fi
    done
    read -p "Enter baraye bazgasht..."
}

check_deps
while true; do
    show_header
    echo -e "  ${C}[1]${NC} Live Packet Analyzer      ${C}[5]${NC} DC Identity Sniper"
    echo -e "  ${C}[2]${NC} Domain/SNI Tracker       ${C}[6]${NC} DPI Flag Sniper"
    echo -e "  ${C}[3]${NC} Tunnel Health Check      ${C}[7]${NC} Triple-Hop Latency"
    echo -e "  ${C}[4]${NC} Cloudflare IP Sweeper    ${C}[8]${NC} MTU Path Discovery"
    echo -e "  ${R}[0]${NC} Exit Engine"
    echo ""
    read -p "Spectre Engine Mode > " choice
    case $choice in
        1) module_1 ;; 2) module_2 ;; 3) module_3 ;; 4) module_4 ;;
        5) module_5 ;; 6) module_6 ;; 7) module_7 ;; 8) module_8 ;;
        0) clear; exit 0 ;;
        *) echo -e "${R}Gozine Eshtebah!${NC}"; sleep 1 ;;
    esac
done
