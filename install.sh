#!/usr/bin/env bash
set -euo pipefail

VERSION="2.3"
GO_VERSION="1.22.4"
REPO_URL="https://github.com/T2HASH/DPISCANING-T2HASH
"
RAW_URL="https://raw.githubusercontent.com/T2HASH/DPISCANING-T2HASH
/main"
INSTALL_DIR="/usr/local"
BINARY_NAME="t2hash-scanner"
SERVICE_NAME="t2hash-scanner"
DEFAULT_PORT="8080"
WORK_DIR="/opt/t2hash-scanner"
MODULE_NAME="t2hash-scanner"

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
  --skip-go          Skip Go installation
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
  echo -e "${DIM}      +-----------------------------------------+${RESET}"
  echo -e "${DIM}      |${RESET}  ${PINK}>${RESET} YouTube : ${BOLD}@T2hsh${RESET}                  ${DIM}|${RESET}"
  echo -e "${DIM}      |${RESET}  ${CYAN}>${RESET} Telegram: ${BOLD}@T2HASHCHANNEL${RESET}          ${DIM}|${RESET}"
  echo -e "${DIM}      |${RESET}  ${GREEN}>${RESET} GitHub  : ${BOLD}@T2HASH${RESET}                 ${DIM}|${RESET}"
  echo -e "${DIM}      +-----------------------------------------+${RESET}"
  echo
}

log()  { echo -e "${GREEN}[OK]${RESET}   $*"; }
info() { echo -e "${CYAN}[..]${RESET}   $*"; }
warn() { echo -e "${YELLOW}[!!]${RESET}   $*"; }
die()  { echo -e "${RED}[XX]${RESET}   $*"; exit 1; }
step() { echo -e "\n${PURPLE}=== $* ===${RESET}\n"; }

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
    warn "Installing missing deps: ${missing[*]}"
    if command -v apt-get &>/dev/null; then
      apt-get update -qq && apt-get install -y curl tar unzip
    elif command -v yum &>/dev/null; then
      yum install -y curl tar unzip
    elif command -v apk &>/dev/null; then
      apk add --no-cache curl tar unzip
    else
      die "Package manager not found. Install manually: ${missing[*]}"
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

  info "Downloading Go ${GO_VERSION} (${os}/${arch})..."
  curl -fsSL -# "$url" -o "${tmpdir}/${tarball}" || die "Download failed"

  info "Installing Go to ${INSTALL_DIR}..."
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

  log "Go ${GO_VERSION} installed"
}

fetch_source() {
  mkdir -p "$WORK_DIR"
  cd "$WORK_DIR"

  local script_dir
  script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" 2>/dev/null && pwd)"

  if [[ -f "${script_dir}/main.go" ]] && [[ "$script_dir" != "$WORK_DIR" ]]; then
    info "Copying local files to ${WORK_DIR}..."
    cp "${script_dir}/main.go" "$WORK_DIR/"
    [[ -f "${script_dir}/README.md" ]] && cp "${script_dir}/README.md" "$WORK_DIR/"
    log "Files copied"
  elif [[ ! -f "$WORK_DIR/main.go" ]]; then
    info "Downloading source from GitHub..."
    curl -fsSL "${RAW_URL}/main.go" -o "$WORK_DIR/main.go" || die "main.go download failed"
    log "Source downloaded"
  else
    info "Source already present"
  fi

  if [[ ! -f "$WORK_DIR/go.mod" ]]; then
    info "Initializing Go module..."
    cd "$WORK_DIR"
    go mod init "$MODULE_NAME" >/dev/null 2>&1 || die "go mod init failed"
    log "go.mod created"
  fi
}

build_binary() {
  cd "$WORK_DIR"
  info "Building binary (ldflags=-s -w)..."
  go build -ldflags="-s -w" -o "$BINARY_NAME" . || die "Build failed"
  chmod +x "$BINARY_NAME"
  local size
  size=$(du -h "$BINARY_NAME" | cut -f1)
  log "Binary built: ${WORK_DIR}/${BINARY_NAME} (${size})"
}

install_binary() {
  local dest="/usr/local/bin/${BINARY_NAME}"
  info "Installing to ${dest}..."
  cp "${WORK_DIR}/${BINARY_NAME}" "$dest"
  chmod +x "$dest"
  log "Installed: ${dest}"
}

install_systemd_service() {
  local binary_path="/usr/local/bin/${BINARY_NAME}"
  local service_file="/etc/systemd/system/${SERVICE_NAME}.service"

  [[ -f "$binary_path" ]] || die "Binary not found at ${binary_path}"

  info "Creating systemd service on port ${PORT}..."

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
    log "Service is active"
  else
    warn "Service did not start - check logs"
  fi
}

show_finale() {
  local ip
  ip=$(hostname -I 2>/dev/null | awk '{print $1}')
  [[ -z "$ip" ]] && ip="YOUR_SERVER_IP"

  echo
  echo -e "${GREEN}${BOLD}+==========================================================+${RESET}"
  echo -e "${GREEN}${BOLD}|            Installation completed successfully!         |${RESET}"
  echo -e "${GREEN}${BOLD}+==========================================================+${RESET}"
  echo

  if [[ "$INSTALL_SERVICE" == true ]]; then
    echo -e "  ${CYAN}>>${RESET} Web Panel:        ${BOLD}http://${ip}:${PORT}${RESET}"
    echo -e "  ${CYAN}>>${RESET} Service status:   ${DIM}systemctl status ${SERVICE_NAME}${RESET}"
    echo -e "  ${CYAN}>>${RESET} View logs:        ${DIM}journalctl -fu ${SERVICE_NAME}${RESET}"
    echo -e "  ${CYAN}>>${RESET} Stop service:     ${DIM}systemctl stop ${SERVICE_NAME}${RESET}"
    echo -e "  ${CYAN}>>${RESET} Restart:          ${DIM}systemctl restart ${SERVICE_NAME}${RESET}"
  else
    echo -e "  ${CYAN}>>${RESET} Run:              ${BOLD}cd ${WORK_DIR} && ./${BINARY_NAME}${RESET}"
    echo -e "  ${CYAN}>>${RESET} Custom port:      ${BOLD}./${BINARY_NAME} 9090${RESET}"
    echo -e "  ${CYAN}>>${RESET} Install service:  ${DIM}sudo bash install.sh --service --port ${PORT}${RESET}"
  fi

  echo
  echo -e "${DIM}  -----------------------------------------------------${RESET}"
  echo -e "  ${PINK}>${RESET} YouTube : ${BOLD}https://youtube.com/@T2hsh${RESET}"
  echo -e "  ${CYAN}>${RESET} Telegram: ${BOLD}https://t.me/T2HASHCHANNEL${RESET}"
  echo -e "  ${GREEN}>${RESET} GitHub  : ${BOLD}https://github.com/T2HASH${RESET}"
  echo -e "${DIM}  -----------------------------------------------------${RESET}"
  echo
}

main() {
  print_banner

  if [[ "$INSTALL_SERVICE" == true ]] && [[ "$EUID" -ne 0 ]]; then
    die "Service install requires root: sudo bash install.sh --service"
  fi

  step "Checking dependencies"
  check_deps
  log "Base tools ready"

  step "Checking Go"
  if [[ "$SKIP_GO" == true ]]; then
    warn "Skipping Go installation"
  elif go_ok; then
    local v
    v=$(go version | grep -oE 'go[0-9]+\.[0-9]+\.[0-9]+' | head -1)
    log "${v} found"
  else
    warn "Go not installed or version too old"
    if [[ "$EUID" -ne 0 ]]; then
      die "Root required to install Go"
    fi
    install_go
  fi

  step "Preparing source"
  fetch_source

  step "Building"
  build_binary

  if [[ "$INSTALL_SERVICE" == true ]]; then
    step "Installing as service"
    install_binary
    install_systemd_service
  fi

  show_finale
}

main "$@"
