<div align="center">

```
 ████████╗██████╗ ██╗  ██╗ █████╗ ███████╗██╗  ██╗    ███████╗ ██████╗ █████╗ ███╗   ██╗███╗   ██╗███████╗██████╗
    ██╔══╝╚════██╗██║  ██║██╔══██╗██╔════╝██║  ██║    ██╔════╝██╔════╝██╔══██╗████╗  ██║████╗  ██║██╔════╝██╔══██╗
    ██║    █████╔╝███████║███████║███████╗███████║    ███████╗██║     ███████║██╔██╗ ██║██╔██╗ ██║█████╗  ██████╔╝
    ██║    ╚═══██╗██╔══██║██╔══██║╚════██║██╔══██║    ╚════██║██║     ██╔══██║██║╚██╗██║██║╚██╗██║██╔══╝  ██╔══██╗
    ██║   ██████╔╝██║  ██║██║  ██║███████║██║  ██║    ███████║╚██████╗██║  ██║██║ ╚████║██║ ╚████║███████╗██║  ██║
    ╚═╝   ╚═════╝ ╚═╝  ╚═╝╚═╝  ╚═╝╚══════╝╚═╝  ╚═╝    ╚══════╝ ╚═════╝╚═╝  ╚═╝╚═╝  ╚═══╝╚═╝  ╚═══╝╚══════╝╚═╝  ╚═╝
```

**اسکنر IP با کارایی بالا و رابط وب real-time**  
نوشته‌شده با Go خالص — بدون dependency، یک باینری، همه‌جا اجرا می‌شه.

[![Go Version](https://img.shields.io/badge/Go-1.18+-00ADD8?style=flat-square&logo=go&logoColor=white)](https://go.dev)
[![License](https://img.shields.io/badge/License-MIT-00ff88?style=flat-square)](LICENSE)
[![Platform](https://img.shields.io/badge/Platform-Linux%20%7C%20macOS-0088ff?style=flat-square)]()
[![Author](https://img.shields.io/badge/Author-t2hash-9b5de5?style=flat-square)](https://github.com/t2hash)

</div>

---

## درباره پروژه

t2hash-scanner یه اسکنر شبکه‌ی self-contained هست که یه رابط وب از طریق HTTP سرو می‌کنه. رنج‌های CIDR یا IP های تکی رو اسکن می‌کنه، پورت‌های TCP رو به‌صورت همزمان تست می‌کنه، latency هر IP رو اندازه می‌گیره، و اختیاری HTTP/HTTPS ریسپانس رو چک می‌کنه — همه‌چیز real-time از طریق Server-Sent Events به مرورگر استریم می‌شه.

هیچ کتابخانه خارجی، npm یا Docker نیاز نیست. یه `go build` یه باینری قابل حمل می‌سازه.

---

## قابلیت‌ها

- **اسکن همزمان** — pool قابل تنظیم goroutine، تا ۵۰۰۰ worker موازی
- **پشتیبانی از CIDR و IP تکی** — چند رنج در یه session، deduplication خودکار
- **range پورت** — ورودی مثل `8000-8010` کنار پورت‌های جداگانه
- **UI real-time** — نتایج از طریق SSE استریم می‌شن بدون polling، latency زیر ثانیه
- **آمار زنده** — IPs/sec، تعداد alive، زمان گذشته، نوار پیشرفت
- **پروب HTTP** — درخواست HEAD اختیاری با ثبت response code و پشتیبانی TLS
- **رنگ‌بندی latency** — سبز < 150ms / زرد < 500ms / قرمز > 500ms
- **جدول قابل sort** — روی هر ستون کلیک کن (IP، latency، پورت‌ها، وضعیت)
- **فیلتر زنده** — نتایج رو با substring IP فیلتر کن بدون reload
- **حالت shuffle** — ترتیب اسکن رو برای توزیع بار رندوم کن
- **دانلود CSV و JSON** — با یه کلیک تمام نتایج رو دانلود کن
- **preset های آماده** — Cloudflare (همه رنج‌ها)، Gcore، Fastly، و preset پورت برای Web / Xray / WARP / Common
- **stop تمیز** — لغو وسط scan، نتایج جزئی حفظ می‌شن
- **یه فایل** — کل backend + frontend در یه `main.go`، بدون پوشه asset

---

## شروع سریع

```bash
git clone https://github.com/t2hash/t2hash-scanner
cd t2hash-scanner
go run main.go
```

مرورگر رو باز کن: **http://localhost:8080**

برای پورت دلخواه:

```bash
go run main.go 9000
```

---

## نصب

### نصب خودکار (لینوکس)

```bash
curl -fsSL https://raw.githubusercontent.com/t2hash/t2hash-scanner/main/install.sh | bash
```

این اسکریپت architecture رو detect می‌کنه، اگه Go نداشت نصب می‌کنه، باینری می‌سازه، و اختیاری یه systemd service ثبت می‌کنه.

### build دستی

```bash
git clone https://github.com/t2hash/t2hash-scanner
cd t2hash-scanner

go build -o t2hash-scanner .

./t2hash-scanner          # روی :8080
./t2hash-scanner 9090     # پورت دلخواه
```

### اجرا به‌عنوان سرویس پس‌زمینه

```bash
sudo bash install.sh --service --port 8080
systemctl status t2hash-scanner
```

---

## تنظیمات

همه چیز از طریق UI قابل تنظیمه. فایل config لازم نیست.

| فیلد | پیش‌فرض | توضیح |
|---|---|---|
| CIDR / IPs | — | رنج‌های جداشده با خط جدید یا IP تکی. خطوطی که با `#` شروع می‌شن نادیده گرفته می‌شن. |
| Ports | `80,443,8080,8443,2053,2083,2086,2087,2096` | comma-separated. از range مثل `8000-8010` پشتیبانی می‌کنه. |
| Concurrency | `500` | تعداد goroutine های همزمان. حداکثر ۵۰۰۰. |
| Timeout | `2000 ms` | timeout TCP برای هر connection. |
| Shuffle IPs | روشن | ترتیب اسکن رو رندوم می‌کنه. |
| Only alive | روشن | IP های unreachable رو از نتایج پنهان می‌کنه. |
| Check HTTP | خاموش | یه درخواست HEAD به پورت‌های باز می‌فرسته و response code رو ثبت می‌کنه. |

---

## Preset های پورت

| Preset | پورت‌ها |
|---|---|
| Web | `80، 443، 8080، 8443` |
| Xray / CF | `80، 443، 2053، 2083، 2086، 2087، 2096، 8080، 8443، 8880` |
| WARP | `500، 854، 859، 864، 878، 880، 2408 و...` |
| Extended | `80، 443، رنج 2052-2096، 8080-8888` |
| Common | `21، 22، 25، 53، 80، 443، 3306، 3389، 6379، 8080، 27017` |

---

## REST API

اگه می‌خوای integrate کنی یا اسکریپت بنویسی:

**شروع اسکن**
```
POST /api/scan/start
Content-Type: application/json

{
  "cidrs":        "104.16.0.0/13\n172.64.0.0/13",
  "ports":        "80,443,8080",
  "timeout_ms":   2000,
  "concurrency":  500,
  "shuffle":      true,
  "only_alive":   true,
  "check_http":   false
}
```

**توقف اسکن**
```
POST /api/scan/stop
```

**وضعیت**
```
GET /api/status
→ { "active": true }
```

**دریافت نتایج real-time (SSE)**
```
GET /events
```

نوع event ها: `result`، `stats`، `done`

---

## پیش‌نیازها

- Go نسخه 1.18 یا بالاتر
- هر سیستم لینوکس / macOS (ویندوز هم با `go run` کار می‌کنه)
- هیچ کتابخانه خارجی — فقط standard library

---

## لایسنس

MIT © 2024 — [t2hash](https://github.com/t2hash)
