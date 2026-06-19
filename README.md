<div align="center">

```
████████╗██████╗ ██╗  ██╗ █████╗ ███████╗██╗  ██╗
╚══██╔══╝╚════██╗██║  ██║██╔══██╗██╔════╝██║  ██║
   ██║    █████╔╝███████║███████║███████╗███████║
   ██║   ██╔═══╝ ██╔══██║██╔══██║╚════██║██╔══██║
   ██║   ███████╗██║  ██║██║  ██║███████║██║  ██║
   ╚═╝   ╚══════╝╚═╝  ╚═╝╚═╝  ╚═╝╚══════╝╚═╝  ╚═╝
        S  C  A  N  N  E  R    v2.3
```

### ⚡ اسکنر IP حرفه‌ای + تست خودکار CDN با Xray ⚡
### رابط وب ۳D، تشخیص خودکار ISP، تولید کانفیگ اتومات

<br/>

[![Version](https://img.shields.io/badge/version-2.3-00ff88?style=for-the-badge&logo=semver&logoColor=white&labelColor=050b14)](https://github.com/T2HASH/t2hash-scanner)
[![Go](https://img.shields.io/badge/Go-1.18+-00c8ff?style=for-the-badge&logo=go&logoColor=white&labelColor=050b14)](https://go.dev)
[![License](https://img.shields.io/badge/license-MIT-8b5cf6?style=for-the-badge&logoColor=white&labelColor=050b14)](LICENSE)
[![Platform](https://img.shields.io/badge/platform-Linux%20%7C%20macOS-fbbf24?style=for-the-badge&logo=linux&logoColor=white&labelColor=050b14)]()
[![Stars](https://img.shields.io/github/stars/T2HASH/t2hash-scanner?style=for-the-badge&logo=github&logoColor=white&color=f472b6&labelColor=050b14)](https://github.com/T2HASH/t2hash-scanner/stargazers)

<br/>

[![YouTube](https://img.shields.io/badge/YouTube-@T2hsh-FF0000?style=for-the-badge&logo=youtube&logoColor=white&labelColor=050b14)](https://youtube.com/@T2hsh)
[![Telegram](https://img.shields.io/badge/Telegram-@T2HASHCHANNEL-29A8EB?style=for-the-badge&logo=telegram&logoColor=white&labelColor=050b14)](https://t.me/T2HASHCHANNEL)
[![GitHub](https://img.shields.io/badge/GitHub-@T2HASH-181717?style=for-the-badge&logo=github&logoColor=white&labelColor=050b14)](https://github.com/T2HASH)

<br/>

```
╔══════════════════════════════════════════════════════════════╗
║   نصب در یک خط — Install in one command                      ║
╚══════════════════════════════════════════════════════════════╝
```

```bash
bash <(curl -fsSL https://raw.githubusercontent.com/T2HASH/t2hash-scanner/main/install.sh)
```

</div>

<br/>

---

<div align="center">

## 🎯 درباره پروژه

</div>

**t2hash-scanner** یه ابزار کامل شبکه‌ای هست که با **Go خالص** نوشته شده، بدون هیچ dependency خارجی. این اسکنر دو حالت اصلی داره:

<table>
<tr>
<td width="50%" valign="top">

### ⚡ TCP Scan Mode
اسکن سریع رنج‌های CIDR یا IP های تکی  
چک کردن پورت‌های باز با concurrency بالا  
اندازه‌گیری latency دقیق میکروثانیه‌ای  
بررسی اختیاری HTTP response code  
خروجی real-time روی UI با SSE

</td>
<td width="50%" valign="top">

### ⬡ Xray CDN Probe Mode
ساخت اتوماتیک کانفیگ VLESS  
تست واقعی traffic از طریق هسته Xray  
تشخیص اینکه IP **واقعاً** کار می‌کنه یا فقط ping میده  
تولید لینک v2rayN/v2rayNG اتومات  
پشتیبانی WebSocket / gRPC / TCP + Reality

</td>
</tr>
</table>

<br/>

---

<div align="center">

## 🚀 قابلیت‌های نسخه ۲.۳

</div>

```
┌─────────────────────────────────────────────────────────────┐
│  🌐  تشخیص خودکار ISP سرور (ایرانسل / همراه اول / مخابرات)  │
│  🎨  رابط کاربری ۳D با particle background و glass morphism  │
│  ⚡  اسکن همزمان تا ۵۰۰۰ goroutine                            │
│  📡  Server-Sent Events برای streaming زنده                  │
│  🔥  دانلود اتوماتیک Xray-core در اولین اجرا                  │
│  📋  Preset های آماده: Cloudflare, Gcore, Fastly, ArvanCloud │
│  🎭  تنظیمات پیشرفته Xray: Fingerprint, Flow, Network        │
│  💾  خروجی CSV و JSON با meta data                           │
│  🎯  لینک vless:// اتومات برای هر IP کاری                    │
│  ⏱   فیلتر زنده + Sort + Search                              │
│  🛡  Graceful stop, کنترل کامل وسط اسکن                      │
└─────────────────────────────────────────────────────────────┘
```

<br/>

---

<div align="center">

## ⚙️ نصب و راه‌اندازی

</div>

### 🔥 روش ۱ — نصب خودکار (پیشنهادی)

این اسکریپت:
- معماری سرورت رو شناسایی می‌کنه (amd64 / arm64)
- اگه Go نداشت، **خودکار** نصبش می‌کنه
- فایل‌ها رو دانلود می‌کنه و باینری می‌سازه
- اختیاری سرویس systemd ثبت می‌کنه

```bash
bash <(curl -fsSL https://raw.githubusercontent.com/T2HASH/t2hash-scanner/main/install.sh)
```

برای نصب به‌عنوان سرویس پس‌زمینه:

```bash
sudo bash <(curl -fsSL https://raw.githubusercontent.com/T2HASH/t2hash-scanner/main/install.sh) --service --port 8080
```

### 🛠 روش ۲ — نصب دستی

```bash
git clone https://github.com/T2HASH/t2hash-scanner.git
cd t2hash-scanner
go build -o t2hash-scanner .
./t2hash-scanner
```

سپس مرورگر رو باز کن:

```
http://YOUR_SERVER_IP:8080
```

<br/>

---

<div align="center">

## 📖 راهنمای استفاده

</div>

<details>
<summary><b>🔹 حالت TCP Scan</b></summary>

<br/>

۱. توی textarea رنج CIDR یا IP رو وارد کن:
```
104.16.0.0/13
172.64.0.0/13
8.8.8.8
```

۲. یا از Preset های آماده استفاده کن:
- `CF All` — همه رنج‌های Cloudflare
- `Gcore` — رنج‌های Gcore CDN
- `Fastly` — رنج‌های Fastly CDN
- `ArvanCloud` — رنج‌های آروان‌کلود

۳. پورت‌ها رو انتخاب کن (یا از Preset):
- `Web` → 80, 443, 8080, 8443
- `Xray/CF` → پورت‌های پشتیبانی شده توسط Cloudflare
- `WARP` → پورت‌های Cloudflare WARP

۴. Concurrency و Timeout رو تنظیم کن

۵. دکمه **▶ START** رو بزن

</details>

<details>
<summary><b>🔸 حالت Xray Probe</b></summary>

<br/>

این حالت فرق اساسی با TCP scan داره — به جای فقط پینگ کردن، **واقعاً ترافیک رد می‌کنه** تا مطمئن شه CDN روی IP کار می‌کنه.

**مراحل:**

۱. وارد تب **⬡ XRAY PROBE** شو

۲. اطلاعات سرور Xray خودت رو وارد کن:
```
UUID:           xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx
Server / SNI:   s1.example.com
Port:           443
WS Path:        /vless
```

۳. تنظیمات پیشرفته رو انتخاب کن:
- **Network**: WebSocket / gRPC / TCP
- **Security**: TLS / Reality / None
- **Fingerprint**: Chrome / Firefox / Safari / Random
- **Flow**: xtls-rprx-vision

۴. **▶ START PROBE** رو بزن

۵. خروجی:
- ✅ IP هایی که علامت **⬡ WORKING** دارن → ترافیک واقعی رد می‌کنن
- ❌ IP هایی که **✕ FAILED** هستن → فقط TCP جواب می‌دن (به درد نمی‌خورن)

۶. روی دکمه **⎘ copy cfg** بزن تا لینک `vless://` اتومات کپی شه

</details>

<details>
<summary><b>🌐 تشخیص ISP خودکار</b></summary>

<br/>

نسخه ۲.۳ به صورت اتومات ISP سرورت رو تشخیص می‌ده و توی بنر بالا با رنگ‌بندی نشون می‌ده:

| ISP | رنگ بنر |
|------|----------|
| Irancell | 🩷 صورتی |
| همراه اول (MCI) | 🔵 آبی |
| مخابرات (TCI) | 🟠 نارنجی |
| Shatel | 🟢 سبز |
| Asiatech | 🟦 سیان |
| Hetzner | 🔴 قرمز |

این یعنی **نتایج اسکن از دیدگاه همون ISP** هستن. اگه سرورت ایرانسله، IP هایی که پیدا می‌کنی روی ایرانسل کار می‌کنن. اگه همراه اوله، روی همراه اول.

</details>

<br/>

---

<div align="center">

## 🔌 REST API

</div>

| Method | Endpoint | توضیح |
|--------|----------|--------|
| `POST` | `/api/scan/start` | شروع TCP scan |
| `POST` | `/api/probe/start` | شروع Xray probe |
| `POST` | `/api/scan/stop` | توقف عملیات |
| `GET` | `/api/status` | وضعیت فعلی |
| `GET` | `/api/isp` | تشخیص ISP سرور |
| `GET` | `/events` | استریم SSE |

<details>
<summary><b>📤 نمونه درخواست TCP Scan</b></summary>

```bash
curl -X POST http://localhost:8080/api/scan/start \
  -H "Content-Type: application/json" \
  -d '{
    "cidrs":        "104.16.0.0/13",
    "ports":        "80,443,2053,2083",
    "timeout_ms":   2000,
    "concurrency":  500,
    "shuffle":      true,
    "only_alive":   true,
    "check_http":   false
  }'
```

</details>

<details>
<summary><b>📤 نمونه درخواست Xray Probe</b></summary>

```bash
curl -X POST http://localhost:8080/api/probe/start \
  -H "Content-Type: application/json" \
  -d '{
    "uuid":         "xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx",
    "server_addr":  "s1.example.com",
    "server_port":  443,
    "sni":          "s1.example.com",
    "ws_path":      "/vless",
    "test_url":     "http://cp.cloudflare.com/generate_204",
    "concurrency":  10,
    "timeout_ms":   8000,
    "auto_ranges":  true
  }'
```

</details>

<br/>

---

<div align="center">

## 🏗 ساختار پروژه

</div>

```
t2hash-scanner/
│
├── main.go              ← کل سورس Go + Frontend (single file)
├── install.sh           ← اسکریپت نصب اتومات
├── README.md            ← همین فایل
└── LICENSE              ← لایسنس MIT
```

<br/>

---

<div align="center">

## ⚙️ تنظیمات قابل تغییر

</div>

| پارامتر | محدوده | پیش‌فرض | توضیح |
|----------|---------|----------|--------|
| `concurrency` | 1 - 5000 | 500 | تعداد goroutine های همزمان |
| `timeout_ms` | 100 - 30000 | 2000 | timeout هر اتصال |
| `xray_concurrency` | 1 - 50 | 10 | همزمانی probe (سنگین‌تره) |
| `xray_timeout_ms` | 2000 - 30000 | 8000 | timeout probe |

<br/>

---

<div align="center">

## 🎓 توصیه‌های حرفه‌ای

</div>

```diff
+ برای پیدا کردن IP تمیز Cloudflare:
+   1. تب TCP scan → preset "CF All" → preset پورت "Xray/CF"
+   2. Concurrency: 800-1500 (بسته به سرورت)
+   3. بعد از تموم شدن → JSON خروجی رو دانلود کن

+ برای تست واقعی IP های Cloudflare با Xray:
+   1. تب Xray Probe → UUID و SNI سرور خودت
+   2. "همه رنج‌های CF (auto)" رو فعال بذار
+   3. Concurrency: 5-15 (probe سنگینه)
+   4. صبر کن تا 100-300 IP پیدا کنه

- نکنه:
-   • Concurrency بالاتر از ۵۰ برای Xray probe (سرور خفه می‌شه)
-   • Timeout کمتر از ۱۵۰۰ms برای CDN (false negative می‌ده)
-   • اسکن از سرور با bandwidth کم
```

<br/>

---

<div align="center">

## 🐛 خطاهای رایج

</div>

<details>
<summary><b>❌ <code>address already in use</code></b></summary>

```bash
# ببین کی پورت رو گرفته
lsof -i :8080

# بکشش
kill $(lsof -ti :8080)

# یا با پورت دیگه اجرا کن
go run main.go 9090
```

</details>

<details>
<summary><b>❌ <code>xray binary not found</code></b></summary>

نسخه ۲.۳ خودش xray رو دانلود می‌کنه ولی اگه به هر دلیلی failed شد:

```bash
# دستی دانلود کن
wget https://github.com/XTLS/Xray-core/releases/latest/download/Xray-linux-64.zip
unzip Xray-linux-64.zip
mv xray /usr/local/bin/
chmod +x /usr/local/bin/xray
```

</details>

<details>
<summary><b>❌ همه probe ها FAILED می‌شن</b></summary>

- مطمئن شو UUID و SNI و WS Path **دقیقاً** با تنظیمات سرور Xray یکیه
- بررسی کن سرور Xray واقعاً فعاله: `systemctl status xray`
- TestURL رو تغییر بده به یه چیز سبک‌تر
- Timeout رو بیار بالا (15000ms)

</details>

<br/>

---

<div align="center">

## 🌟 پشتیبانی و ارتباط

<br/>

<a href="https://youtube.com/@T2hsh">
<img src="https://img.shields.io/badge/YouTube-FF0000?style=for-the-badge&logo=youtube&logoColor=white" alt="YouTube"/>
</a>
<a href="https://t.me/T2HASHCHANNEL">
<img src="https://img.shields.io/badge/Telegram-29A8EB?style=for-the-badge&logo=telegram&logoColor=white" alt="Telegram"/>
</a>
<a href="https://github.com/T2HASH">
<img src="https://img.shields.io/badge/GitHub-181717?style=for-the-badge&logo=github&logoColor=white" alt="GitHub"/>
</a>

<br/><br/>

| پلتفرم | لینک |
|---------|------|
| 📺 YouTube | [`@T2hsh`](https://youtube.com/@T2hsh) |
| ✈️ Telegram Channel | [`@T2HASHCHANNEL`](https://t.me/T2HASHCHANNEL) |
| 💻 GitHub | [`@T2HASH`](https://github.com/T2HASH) |

</div>

<br/>

---

<div align="center">

## 📜 لایسنس

این پروژه تحت لایسنس **MIT** منتشر شده.  
آزادانه می‌تونی استفاده کنی، fork کنی، تغییر بدی.

<br/>

```
╔══════════════════════════════════════════════════════════════╗
║                                                              ║
║      Made with ⚡ by t2hash                                   ║
║      Star ⭐ this repo if you find it useful                  ║
║                                                              ║
╚══════════════════════════════════════════════════════════════╝
```

</div>
