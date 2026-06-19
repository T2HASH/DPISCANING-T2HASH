#!/usr/bin/env bash
set -euo pipefail

VERSION="2.3"
GO_VERSION="1.22.4"
REPO_URL="https://github.com/T2HASH/t2hash-scanner"
RAW_URL="https://raw.githubusercontent.com/T2HASH/t2hash-scanner/main"
INSTALL_DIR="/usr/local"
BINARY_NAME="t2hash-scanner"
SERVICE_NAME="t2hash-scanner"
DEFAULT_PORT="8080"
WORK_DIR="/opt/t2hash-scanner"

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
CYAN='\033[0;36m'
PURPLE='\033[0;35m'
PINK='\033[1;35m'
BOLD='\033[1m'
DIM='\033[2m'
RESET='\033[0m'

INSTALL_SERVICE=false
PORT="$DEFAULT_PORT"
SKIP_GO=false

for arg in "$@"; do
  case "$arg" in
    --service)   INSTALL_SERVICE=true ;;
    --port=*)    PORT="${arg#--port=}" ;;
    --port)      shift; PORT="${1:-$DEFAULT_PORT}" ;;
    --skip-go)   SKIP_GO=true ;;
    --help|-h)
      cat <<HLP
t2hash-scanner installer v${VERSION}

Usage: bash install.sh [options]

Options:
  --service          Install as a systemd service
  --port PORT        Custom port (default: 8080)
  --skip-go          Don't install Go (use existing)
  -h, --help         Show this help

Examples:
  bash install.sh
  sudo bash install.sh --service --port 8080
HLP
      exit 0 ;;
  esac
done

print_banner() {
  echo
  echo -e "${CYAN}${BOLD}"
  cat <<'BANNER'
   ████████╗██████╗ ██╗  ██╗ █████╗ ███████╗██╗  ██╗
   ╚══██╔══╝╚════██╗██║  ██║██╔══██╗██╔════╝██║  ██║
      ██║    █████╔╝███████║███████║███████╗███████║
      ██║   ██╔═══╝ ██╔══██║██╔══██║╚════██║██╔══██║
      ██║   ███████╗██║  ██║██║  ██║███████║██║  ██║
      ╚═╝   ╚══════╝╚═╝  ╚═╝╚═╝  ╚═╝╚══════╝╚═╝  ╚═╝
BANNER
  echo -e "${PURPLE}              S C A N N E R   v${VERSION}${RESET}"
  echo
  echo -e "${DIM}      ┌─────────────────────────────────────────┐${RESET}"
  echo -e "${DIM}      │${RESET}  ${PINK}▶${RESET} YouTube : ${BOLD}@T2hsh${RESET}                  ${DIM}│${RESET}"
  echo -e "${DIM}      │${RESET}  ${CYAN}✈${RESET} Telegram: ${BOLD}@T2HASHCHANNEL${RESET}          ${DIM}│${RESET}"
  echo -e "${DIM}      │${RESET}  ${GREEN}⌥${RESET} GitHub  : ${BOLD}@T2HASH${RESET}                 ${DIM}│${RESET}"
  echo -e "${DIM}      └─────────────────────────────────────────┘${RESET}"
  echo
}

log()  { echo -e "${GREEN}[✓]${RESET} $*"; }
info() { echo -e "${CYAN}[→]${RESET} $*"; }
warn() { echo -e "${YELLOW}[!]${RESET} $*"; }
die()  { echo -e "${RED}[✗]${RESET} $*"; exit 1; }
step() { echo -e "\n${PURPLE}━━━ $* ━━━${RESET}\n"; }

detect_arch() {
  case "$(uname -m)" in
    x86_64)  echo "amd64" ;;
    aarch64) echo "arm64" ;;
    armv7l)  echo "armv6l" ;;
    *)       die "Unsupported architecture: $(uname -m)" ;;
  esac
}

detect_os() {
  case "$(uname -s)" in
    Linux)  echo "linux" ;;
    Darwin) echo "darwin" ;;
    *)      die "Unsupported OS" ;;
  esac
}

check_deps() {
  local missing=()
  for cmd in curl tar; do
    command -v "$cmd" &>/dev/null || missing+=("$cmd")
  done
  if [[ ${#missing[@]} -gt 0 ]]; then
    warn "نصب پیش‌نیازها: ${missing[*]}"
    if command -v apt-get &>/dev/null; then
      apt-get update -qq && apt-get install -y "${missing[@]}" curl tar
    elif command -v yum &>/dev/null; then
      yum install -y "${missing[@]}" curl tar
    elif command -v apk &>/dev/null; then
      apk add --no-cache "${missing[@]}" curl tar
    else
      die "Package manager پیدا نشد. دستی نصب کن: ${missing[*]}"
    fi
  fi
}

go_ok() {
  command -v go &>/dev/null || return 1
  local ver
  ver=$(go version 2>/dev/null | grep -oE 'go[0-9]+\.[0-9]+' | head -1 | sed 's/go//')
  [[ -z "$ver" ]] && return 1
  local major minor
  major="$(echo "$ver" | cut -d. -f1)"
  minor="$(echo "$ver" | cut -d. -f2)"
  [[ "$major" -ge 1 && "$minor" -ge 18 ]]
}

install_go() {
  local os arch tarball url tmpdir

  os="$(detect_os)"
  arch="$(detect_arch)"
  tarball="go${GO_VERSION}.${os}-${arch}.tar.gz"
  url="https://go.dev/dl/${tarball}"
  tmpdir="$(mktemp -d)"

  info "دانلود Go ${GO_VERSION} برای ${os}/${arch}..."
  curl -fsSL -# "$url" -o "${tmpdir}/${tarball}" || die "دانلود ناموفق"

  info "نصب Go در ${INSTALL_DIR}..."
  rm -rf "${INSTALL_DIR}/go"
  tar -C "$INSTALL_DIR" -xzf "${tmpdir}/${tarball}"
  rm -rf "$tmpdir"

  export PATH="${INSTALL_DIR}/go/bin:$PATH"

  if ! grep -q "${INSTALL_DIR}/go/bin" /etc/profile 2>/dev/null; then
    echo "export PATH=\$PATH:${INSTALL_DIR}/go/bin" >> /etc/profile
  fi
  for rc in "$HOME/.bashrc" "$HOME/.zshrc"; do
    if [[ -f "$rc" ]] && ! grep -q "${INSTALL_DIR}/go/bin" "$rc"; then
      echo "export PATH=\$PATH:${INSTALL_DIR}/go/bin" >> "$rc"
    fi
  done

  log "Go ${GO_VERSION} نصب شد"
}

fetch_source() {
  mkdir -p "$WORK_DIR"
  cd "$WORK_DIR"

  local script_dir
  script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" 2>/dev/null && pwd)"

  if [[ -f "${script_dir}/main.go" ]] && [[ "$script_dir" != "$WORK_DIR" ]]; then
    info "کپی فایل‌های لوکال به ${WORK_DIR}..."
    cp "${script_dir}/main.go" "$WORK_DIR/"
    [[ -f "${script_dir}/README.md" ]] && cp "${script_dir}/README.md" "$WORK_DIR/"
    log "فایل‌ها کپی شدن"
    return
  fi

  if [[ ! -f "$WORK_DIR/main.go" ]]; then
    info "دانلود سورس از GitHub..."
    curl -fsSL "${RAW_URL}/main.go" -o "$WORK_DIR/main.go" || die "دانلود main.go ناموفق"
    log "سورس دانلود شد"
  else
    info "سورس از قبل موجوده"
  fi
}

build_binary() {
  cd "$WORK_DIR"
  info "ساخت باینری (ldflags=-s -w)..."
  go build -ldflags="-s -w" -o "$BINARY_NAME" . || die "Build ناموفق"
  chmod +x "$BINARY_NAME"
  local size
  size=$(du -h "$BINARY_NAME" | cut -f1)
  log "باینری ساخته شد: ${WORK_DIR}/${BINARY_NAME} (${size})"
}

install_binary() {
  local dest="/usr/local/bin/${BINARY_NAME}"
  info "نصب در ${dest}..."
  cp "${WORK_DIR}/${BINARY_NAME}" "$dest"
  chmod +x "$dest"
  log "نصب کامل شد: ${dest}"
}

install_systemd_service() {
  local binary_path="/usr/local/bin/${BINARY_NAME}"
  local service_file="/etc/systemd/system/${SERVICE_NAME}.service"

  [[ -f "$binary_path" ]] || die "باینری پیدا نشد در ${binary_path}"

  info "ساخت systemd service روی پورت ${PORT}..."

  cat > "$service_file" <<EOF
[Unit]
Description=t2hash-scanner v${VERSION} - IP Scanner + Xray Probe
Documentation=${REPO_URL}
After=network.target
Wants=network.target

[Service]
Type=simple
ExecStart=${binary_path} ${PORT}
Restart=on-failure
RestartSec=5
User=root
WorkingDirectory=${WORK_DIR}
StandardOutput=journal
StandardError=journal
NoNewPrivileges=false
LimitNOFILE=65535

[Install]
WantedBy=multi-user.target
EOF

  systemctl daemon-reload
  systemctl enable "$SERVICE_NAME" 2>/dev/null
  systemctl restart "$SERVICE_NAME"

  sleep 2
  if systemctl is-active --quiet "$SERVICE_NAME"; then
    log "سرویس فعال شد"
  else
    warn "سرویس راه نیفتاد — لاگ‌ها رو ببین"
  fi
}

show_finale() {
  local ip
  ip=$(hostname -I 2>/dev/null | awk '{print $1}')
  [[ -z "$ip" ]] && ip="YOUR_SERVER_IP"

  echo
  echo -e "${GREEN}${BOLD}╔══════════════════════════════════════════════════════════╗${RESET}"
  echo -e "${GREEN}${BOLD}║          ✓  نصب با موفقیت کامل شد!                     ║${RESET}"
  echo -e "${GREEN}${BOLD}╚══════════════════════════════════════════════════════════╝${RESET}"
  echo

  if [[ "$INSTALL_SERVICE" == true ]]; then
    echo -e "  ${CYAN}🌐 آدرس وب پنل:${RESET}     ${BOLD}http://${ip}:${PORT}${RESET}"
    echo -e "  ${CYAN}📊 وضعیت سرویس:${RESET}    ${DIM}systemctl status ${SERVICE_NAME}${RESET}"
    echo -e "  ${CYAN}📝 مشاهده لاگ:${RESET}     ${DIM}journalctl -fu ${SERVICE_NAME}${RESET}"
    echo -e "  ${CYAN}⏹  توقف سرویس:${RESET}     ${DIM}systemctl stop ${SERVICE_NAME}${RESET}"
    echo -e "  ${CYAN}🔄 ریستارت:${RESET}        ${DIM}systemctl restart ${SERVICE_NAME}${RESET}"
  else
    echo -e "  ${CYAN}▶ اجرا:${RESET}            ${BOLD}cd ${WORK_DIR} && ./${BINARY_NAME}${RESET}"
    echo -e "  ${CYAN}▶ پورت دلخواه:${RESET}     ${BOLD}./${BINARY_NAME} 9090${RESET}"
    echo -e "  ${CYAN}▶ نصب به‌عنوان سرویس:${RESET}"
    echo -e "                       ${DIM}sudo bash install.sh --service --port ${PORT}${RESET}"
  fi

  echo
  echo -e "${DIM}  ─────────────────────────────────────────────────────${RESET}"
  echo -e "  ${PINK}▶${RESET} YouTube : ${BOLD}https://youtube.com/@T2hsh${RESET}"
  echo -e "  ${CYAN}✈${RESET} Telegram: ${BOLD}https://t.me/T2HASHCHANNEL${RESET}"
  echo -e "  ${GREEN}⌥${RESET} GitHub  : ${BOLD}https://github.com/T2HASH${RESET}"
  echo -e "${DIM}  ─────────────────────────────────────────────────────${RESET}"
  echo
}

main() {
  print_banner

  if [[ "$INSTALL_SERVICE" == true ]] && [[ "$EUID" -ne 0 ]]; then
    die "برای نصب سرویس باید root باشی: sudo bash install.sh --service"
  fi

  step "بررسی پیش‌نیازها"
  check_deps
  log "ابزارهای پایه آماده‌ست"

  step "بررسی Go"
  if [[ "$SKIP_GO" == true ]]; then
    warn "Skip Go installation"
  elif go_ok; then
    local v
    v=$(go version | grep -oE 'go[0-9]+\.[0-9]+\.[0-9]+' | head -1)
    log "${v} موجوده"
  else
    warn "Go نصب نیست یا نسخه قدیمیه"
    if [[ "$EUID" -ne 0 ]]; then
      die "برای نصب Go باید root باشی"
    fi
    install_go
  fi

  step "آماده‌سازی سورس"
  fetch_source

  step "Build کردن"
  build_binary

  if [[ "$INSTALL_SERVICE" == true ]]; then
    step "نصب به‌عنوان سرویس"
    install_binary
    install_systemd_service
  fi

  show_finale
}

main "$@"
