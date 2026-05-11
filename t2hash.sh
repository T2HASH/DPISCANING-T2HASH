#!/bin/bash

# ==============================================================================
# Project: Multi DPI Scanning
# Creator: t2hash
# YouTube: @T2HSH
# Telegram: https://t.me/t2hashchannel
# GitHub: T2HASH
# ==============================================================================

# --- System Config (Muting Errors & Noise) ---
set +m 2>/dev/null
shopt -s huponexit 2>/dev/null

# --- Colors ---
RED='\033[1;31m'
GREEN='\033[1;32m'
YELLOW='\033[1;33m'
BLUE='\033[1;34m'
CYAN='\033[1;36m'
WHITE='\033[1;37m'
NC='\033[0m'

# --- Check Dependencies ---
check_deps() {
    if ! command -v tcpdump &> /dev/null; then
        echo -e "${RED}[!] tcpdump nasb nist. Dar hal nasb...${NC}"
        sudo apt-get update -y &>/dev/null && sudo apt-get install tcpdump -y &>/dev/null
    fi
    if ! command -v curl &> /dev/null; then
        echo -e "${RED}[!] curl nasb nist. Dar hal nasb...${NC}"
        sudo apt-get install curl -y &>/dev/null
    fi
}

# --- UI Header ---
show_header() {
    clear
    echo -e "${CYAN}╔══════════════════════════════════════════════════════════════╗${NC}"
    echo -e "${CYAN}║${NC} ${WHITE}             M U L T I   D P I   S C A N N I N G            ${NC} ${CYAN}║${NC}"
    echo -e "${CYAN}║${NC} ${YELLOW}                  [ Layer-0 Engine v1.0 ]                   ${NC} ${CYAN}║${NC}"
    echo -e "${CYAN}╠══════════════════════════════════════════════════════════════╣${NC}"
    echo -e "${CYAN}║${NC} ${GREEN}Creator:${NC} t2hash                                              ${CYAN}║${NC}"
    echo -e "${CYAN}║${NC} ${GREEN}YouTube:${NC} @T2HSH                                              ${CYAN}║${NC}"
    echo -e "${CYAN}║${NC} ${GREEN}Telegram:${NC} https://t.me/t2hashchannel                         ${CYAN}║${NC}"
    echo -e "${CYAN}║${NC} ${GREEN}GitHub:${NC} T2HASH                                               ${CYAN}║${NC}"
    echo -e "${CYAN}╚══════════════════════════════════════════════════════════════╝${NC}"
    echo ""
}

# --- Module 1: Live Packet Analyzer ---
module_live_analyzer() {
    show_header
    echo -e "${WHITE}--- [ 1. Live Packet Analyzer ] ---${NC}"
    echo -e "${YELLOW}In bakhsh tamame packet haye voroodi be yek port ra neshon midahad.${NC}\n"
    
    read -p "Che porti ra monitor konim? (e.g. 443): " target_port
    target_port=${target_port:-443}

    echo -e "\n${GREEN}[!] Dar hal monitor kardan port $target_port ...${NC}"
    echo -e "${RED}[!] Baraye khorooj (Ctrl+C) ra bezanid.${NC}\n"

    sudo tcpdump -i any port $target_port -nn -vv -X
}

# --- Module 2: SNI/Domain Tracker ---
module_domain_tracker() {
    show_header
    echo -e "${WHITE}--- [ 2. Domain / SNI Tracker ] ---${NC}"
    echo -e "${YELLOW}Peyda kardan packet hayi ke shamel domain shoma hastand.${NC}\n"

    read -p "Domain khod ra vared konid (e.g. t2hash.site): " my_domain
    read -p "Port ra vared konid (e.g. 443): " my_port
    my_port=${my_port:-443}

    echo -e "\n${GREEN}[!] Dar hal jostojoo baraye domain: $my_domain roye port $my_port ...${NC}"
    echo -e "${RED}[!] Baraye khorooj (Ctrl+C) ra bezanid.${NC}\n"

    sudo tcpdump -i any port $my_port -nn -A 2>/dev/null | grep --line-buffered -i "$my_domain"
}

# --- Module 3: Tunnel Health Analyzer ---
module_tunnel_health() {
    show_header
    echo -e "${WHITE}--- [ 3. Automated Status Analyzer ] ---${NC}"
    echo -e "${YELLOW}Tahlil khodkar vaziyat filter shodan ya vasl shodan tunnel.${NC}\n"

    read -p "Domain (e.g. t2hash.site): " my_domain
    read -p "Port (e.g. 443): " my_port
    my_port=${my_port:-443}

    echo -e "\n${CYAN}[!] Montazer mandan baraye etesal be $my_domain (Max 15s)...${NC}"
    
    TMP_FILE=$(mktemp)
    
    # Safe execution with timeout so it doesn't hang forever
    sudo timeout 15 tcpdump -i any port $my_port -nn -A -c 10 2>/dev/null | grep -i "$my_domain" -B 5 > "$TMP_FILE"
    
    echo -e "\n${WHITE}--- [ Natije Tahlil (Final Report) ] ---${NC}"

    if [ ! -s "$TMP_FILE" ]; then
        echo -e "${RED}[✘] STATUS: Hich packeti be server naresid.${NC}"
        echo -e "${RED}[!] Natije: Ehtemalan port ya domain dar Iran block shode ast.${NC}"
    else
        echo -e "${GREEN}[✔] STATUS: Packet ha ba movafaghiyat daryaft shodand.${NC}"
        
        if grep -q "Flags \[R\]" "$TMP_FILE"; then
            echo -e "${RED}[✘] DETECTION: Etesal tavasot filtering ghat shod (Reset Flag).${NC}"
        fi

        if grep -q "Flags \[S\.\]" "$TMP_FILE"; then
            echo -e "${GREEN}[✔] DETECTION: Handshake ba movafaghiyat anjam shod (SYN-ACK).${NC}"
        fi

        if grep -q "Flags \[P\.\]" "$TMP_FILE"; then
            echo -e "${GREEN}[✔] DETECTION: Data dar hal entaghal ast (Push Flag).${NC}"
            echo -e "${CYAN}[!] Natije: Tunnel shoma bedoon moshkel kar mikonad!${NC}"
        fi
    fi
    rm -f "$TMP_FILE"
    echo -e "\n${YELLOW}Baraye bazgasht be menu, Enter ra bezanid...${NC}"
    read -r
}

# --- Module 4: Deep IP Sweeper ---
module_deep_sweeper() {
    show_header
    echo -e "${WHITE}--- [ 4. Deep Cloudflare Sweeper ] ---${NC}"
    echo -e "${YELLOW}Jostojooye amigh IP haye Cloudflare (Shokhm zadan range ha).${NC}\n"

    tput cnorm
    echo -e "${WHITE}Mesal Range: 188.114 | 104.20 | 172.64 | 190.93${NC}"
    read -p "1. Range paye ra vared konid (e.g. 188.114): " base_ip
    read -p "2. Block shoroo ra vared konid (0-255, e.g. 10): " start_block
    read -p "3. Tedad block baraye scan? (max 20): " blocks_count

    tput civis 
    echo -e "\n${CYAN}[!] Dar hal tolid IP ha...${NC}"

    TMP_IP_LIST=$(mktemp)
    TMP_RESULT=$(mktemp)

    end_block=$((start_block + blocks_count - 1))
    if [ "$end_block" -gt 255 ]; then end_block=255; fi

    for ((c=$start_block; c<=$end_block; c++)); do
        for ((d=1; d<=254; d++)); do
            echo "$base_ip.$c.$d" >> "$TMP_IP_LIST"
        done
    done

    TOTAL_IPS=$(wc -l < "$TMP_IP_LIST")
    echo -e "${YELLOW}[!] Hadaf ghofl shod: Dar hal scan $TOTAL_IPS IP...${NC}\n"

    # Engine Execution (Absolute silence & parallel processing)
    {
        cat "$TMP_IP_LIST" | xargs -P 200 -I {} bash -c '
            TIME=$(curl -o /dev/null -s -w "%{time_connect}\n" --connect-timeout 2.5 http://{});
            if [ "$TIME" != "0.000000" ] && [ -n "$TIME" ]; then
                echo "$TIME {}" >> '"$TMP_RESULT"';
            fi
        ' 
    } &>/dev/null & 

    SCAN_PID=$!
    SPINNER=("⠋" "⠙" "⠹" "⠸" "⠼" "⠴" "⠦" "⠧" "⠇" "⠏")

    while kill -0 $SCAN_PID 2>/dev/null; do
        for frame in "${SPINNER[@]}"; do
            FOUND=$(wc -l < "$TMP_RESULT" 2>/dev/null || echo 0)
            echo -ne "\r${CYAN}[${YELLOW}$frame${CYAN}] Dar hal eskan network $base_ip.$start_block.0 ta $base_ip.$end_block.255 ... ${GREEN}IP haye Salem: $FOUND ${NC}"
            sleep 0.1
        done
    done

    tput cnorm

    echo -e "\n\n${CYAN}====================================================${NC}"
    echo -e "${YELLOW}  🏆 TOP 10 BEST IPs FOUND 🏆${NC}"
    echo -e "${CYAN}====================================================${NC}"

    if [ -s "$TMP_RESULT" ]; then
        sort -n "$TMP_RESULT" | head -n 10 | while read time ip; do
            echo -e "${GREEN} [✔] IP: ${WHITE}$ip${NC}  \t${YELLOW}Ping: ${time}s${NC}"
        done
    else
        echo -e "${RED} [✘] Hich IP salemi dar in block peyda nashod.${NC}"
        echo -e "${YELLOW} [!] Pishnahad: Block haye dige (meslan 20 ta 30) ro test konid.${NC}"
    fi

    rm -f "$TMP_IP_LIST" "$TMP_RESULT"
    echo -e "\n${YELLOW}Baraye bazgasht be menu, Enter ra bezanid...${NC}"
    read -r
}

# --- Main Logic Loop ---
check_deps
while true; do
    show_header
    echo -e "  ${CYAN}[1]${NC} ${WHITE}Live Packet Analyzer (Kaleboshekafi Packet ha)${NC}"
    echo -e "  ${CYAN}[2]${NC} ${WHITE}Domain/SNI Tracker (Peygiri Domain)${NC}"
    echo -e "  ${CYAN}[3]${NC} ${WHITE}Automated Status Analyzer (Tahlil Vaziyat Tunnel)${NC}"
    echo -e "  ${CYAN}[4]${NC} ${WHITE}Deep Cloudflare Sweeper (Peyda kardan IP tamiz)${NC}"
    echo -e "  ${CYAN}[0]${NC} ${RED}Exit (Khorooj)${NC}"
    echo -e "\n${YELLOW}Lotfan yek gozine ra entekhab konid > ${NC}"
    read -p "" choice

    case $choice in
        1) module_live_analyzer ;;
        2) module_domain_tracker ;;
        3) module_tunnel_health ;;
        4) module_deep_sweeper ;;
        0) clear; tput cnorm; echo -e "${GREEN}Ba tashakor az shoma! Channel ma: @t2hashchannel${NC}"; exit 0 ;;
        *) echo -e "${RED}[!] Gozine na-motabar ast!${NC}"; sleep 1 ;;
    esac
done
