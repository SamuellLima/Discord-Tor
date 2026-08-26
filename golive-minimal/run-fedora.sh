#!/usr/bin/env bash
set -Eeuo pipefail

umask 077

readonly APP_ID='com.discordapp.Discord'
readonly SOCKS_HOST='127.0.0.1'
readonly SOCKS_PORT='19060'
readonly SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd -P)"
readonly BANNER_FILE="$SCRIPT_DIR/internal/banner/art.txt"

say() {
    local tag="$1"
    shift
    printf '[%s] [%s] %s\n' "$(date '+%H:%M:%S')" "$tag" "$*"
}

say_err() {
    local tag="$1"
    shift
    printf '[%s] [%s] %s\n' "$(date '+%H:%M:%S')" "$tag" "$*" >&2
}

print_banner() {
    printf '\n'
    if [[ -f "$BANNER_FILE" ]]; then
        cat -- "$BANNER_FILE"
        printf '\n'
    fi
    say 'AVISO' 'Feito de ultima hora. Deve apresentar alguns bugs.'
}

for dependency in tor flatpak curl python3 flock; do
    if ! command -v "$dependency" >/dev/null 2>&1; then
        say_err 'ERRO' "Dependencia ausente: $dependency"
        exit 1
    fi
done

if [[ ${EUID} -eq 0 ]]; then
    say_err 'ERRO' 'Nao execute como root.'
    exit 1
fi

if ! flatpak info "$APP_ID" >/dev/null 2>&1; then
    say_err 'ERRO' "Discord Flatpak ($APP_ID) nao esta instalado."
    exit 1
fi

print_banner

runtime_parent="${XDG_RUNTIME_DIR:-/tmp}"
runtime_dir="$(mktemp -d -- "$runtime_parent/golive-tor.XXXXXX")"
state_parent="${XDG_STATE_HOME:-${HOME:?HOME nao definido}/.local/state}"
state_dir="$state_parent/golive-tor"
mkdir -p -- "$state_dir/tor-data"

tor_pid=''
pac_pid=''
discord_started=0
cleanup() {
    trap - EXIT INT TERM HUP
    if (( discord_started )); then
        flatpak kill "$APP_ID" >/dev/null 2>&1 || true
    fi
    if [[ -n "$pac_pid" ]]; then
        kill "$pac_pid" >/dev/null 2>&1 || true
        wait "$pac_pid" 2>/dev/null || true
    fi
    if [[ -n "$tor_pid" ]]; then
        kill "$tor_pid" >/dev/null 2>&1 || true
        wait "$tor_pid" 2>/dev/null || true
    fi
    rm -rf -- "$runtime_dir"
}
trap cleanup EXIT INT TERM HUP

exec 9>>"$state_dir/launcher.lock"
if ! flock -n 9; then
    say_err 'ERRO' 'Discord-Tor ja esta em execucao.'
    exit 1
fi

say 'INFO' 'Iniciando Discord-Tor.'

if curl --silent --show-error --max-time 1 \
    --proxy "socks5h://$SOCKS_HOST:$SOCKS_PORT" https://gateway.discord.gg/ \
    --output /dev/null 2>/dev/null; then
    say_err 'ERRO' "A porta $SOCKS_PORT ja contem outro proxy SOCKS. Encerre-o antes de continuar."
    exit 1
fi

cat >"$runtime_dir/torrc" <<EOF
SocksPort $SOCKS_HOST:$SOCKS_PORT
SocksPolicy accept 127.0.0.1
SocksPolicy reject *
DataDirectory "$state_dir/tor-data"
ClientOnly 1
AvoidDiskWrites 1
SafeSocks 1
Log notice stdout
EOF

say 'INFO' "Iniciando Tor local em $SOCKS_HOST:$SOCKS_PORT..."
tor -f "$runtime_dir/torrc" >>"$state_dir/tor.log" 2>&1 &
tor_pid=$!

deadline=$((SECONDS + 300))
tor_ready=0
say 'INFO' 'Aguardando SOCKS5 + TLS ate gateway.discord.gg...'
while (( SECONDS < deadline )); do
    if ! kill -0 "$tor_pid" 2>/dev/null; then
        say_err 'ERRO' "Tor encerrou durante a inicializacao. Consulte $state_dir/tor.log"
        exit 1
    fi
    if curl --silent --show-error --max-time 8 --connect-timeout 4 \
        --proxy "socks5h://$SOCKS_HOST:$SOCKS_PORT" https://gateway.discord.gg/ \
        --output /dev/null 2>/dev/null; then
        tor_ready=1
        break
    fi
    sleep 1
done

if (( ! tor_ready )); then
    say_err 'ERRO' 'Tor nao conseguiu acessar o gateway do Discord em 5 minutos.'
    exit 1
fi
say 'OK' 'Tor confirmado: SOCKS5 e TLS do gateway estao validos.'

python3 "$SCRIPT_DIR/cmd/fedora/pac_server.py" "$SOCKS_HOST:$SOCKS_PORT" \
    >"$runtime_dir/pac-port" 2>"$state_dir/pac.log" &
pac_pid=$!

deadline=$((SECONDS + 10))
while [[ ! -s "$runtime_dir/pac-port" ]] && (( SECONDS < deadline )); do
    if ! kill -0 "$pac_pid" 2>/dev/null; then
        say_err 'ERRO' "Servidor PAC local nao iniciou. Consulte $state_dir/pac.log"
        exit 1
    fi
    sleep 0.1
done
if [[ ! -s "$runtime_dir/pac-port" ]]; then
    say_err 'ERRO' 'Servidor PAC local nao informou a porta.'
    exit 1
fi
read -r pac_port <"$runtime_dir/pac-port"
if [[ ! "$pac_port" =~ ^[0-9]+$ ]]; then
    say_err 'ERRO' 'Porta PAC local invalida.'
    exit 1
fi
pac_url="http://127.0.0.1:$pac_port/discord-tor.pac"
curl --silent --show-error --fail --max-time 2 "$pac_url" --output /dev/null
say 'INFO' "PAC local em $pac_url"

say 'INFO' 'Reiniciando Discord Flatpak com PAC local...'
flatpak kill "$APP_ID" >/dev/null 2>&1 || true
sleep 1
flatpak run "$APP_ID" \
    "--proxy-pac-url=$pac_url" \
    "--force-webrtc-ip-handling-policy=default_public_interface_only" \
    >>"$state_dir/discord.log" 2>&1 &
discord_pid=$!
discord_started=1

say 'OK' 'Discord aberto. Apenas discord.gg passa pelo Tor; midia fica direta.'
say 'INFO' 'Mantenha este terminal aberto. Ctrl+C encerra o Discord e o Tor.'
set +e
wait "$discord_pid"
discord_status=$?
set -e
discord_started=0

if (( discord_status != 0 )); then
    say_err 'ERRO' "Discord encerrou com status $discord_status."
    exit "$discord_status"
fi
say 'INFO' 'Discord encerrado; finalizando Tor.'
