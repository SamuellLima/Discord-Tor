#!/usr/bin/env bash
set -Eeuo pipefail

umask 077

readonly APP_ID='com.discordapp.Discord'
readonly SOCKS_HOST='127.0.0.1'
readonly SOCKS_PORT='19060'
readonly SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd -P)"

for dependency in tor flatpak curl python3 flock; do
    if ! command -v "$dependency" >/dev/null 2>&1; then
        printf 'Dependencia ausente: %s\n' "$dependency" >&2
        exit 1
    fi
done

if [[ ${EUID} -eq 0 ]]; then
    printf 'Nao execute como root.\n' >&2
    exit 1
fi

if ! flatpak info "$APP_ID" >/dev/null 2>&1; then
    printf 'Discord Flatpak (%s) nao esta instalado.\n' "$APP_ID" >&2
    exit 1
fi

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
    printf 'Discord-Tor ja esta em execucao.\n' >&2
    exit 1
fi

if curl --silent --show-error --max-time 1 \
    --proxy "socks5h://$SOCKS_HOST:$SOCKS_PORT" https://gateway.discord.gg/ \
    --output /dev/null 2>/dev/null; then
    printf 'A porta %s ja contem outro proxy SOCKS. Encerre-o antes de continuar.\n' "$SOCKS_PORT" >&2
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

printf 'Iniciando Tor local...\n'
tor -f "$runtime_dir/torrc" >>"$state_dir/tor.log" 2>&1 &
tor_pid=$!

deadline=$((SECONDS + 300))
tor_ready=0
while (( SECONDS < deadline )); do
    if ! kill -0 "$tor_pid" 2>/dev/null; then
        printf 'Tor encerrou durante a inicializacao. Consulte %s/tor.log\n' "$state_dir" >&2
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
    printf 'Tor nao conseguiu acessar o gateway do Discord em 5 minutos.\n' >&2
    exit 1
fi
printf 'Tor confirmado: SOCKS5 e TLS do gateway estao validos.\n'

python3 "$SCRIPT_DIR/cmd/fedora/pac_server.py" "$SOCKS_HOST:$SOCKS_PORT" \
    >"$runtime_dir/pac-port" 2>"$state_dir/pac.log" &
pac_pid=$!

deadline=$((SECONDS + 10))
while [[ ! -s "$runtime_dir/pac-port" ]] && (( SECONDS < deadline )); do
    if ! kill -0 "$pac_pid" 2>/dev/null; then
        printf 'Servidor PAC local nao iniciou. Consulte %s/pac.log\n' "$state_dir" >&2
        exit 1
    fi
    sleep 0.1
done
if [[ ! -s "$runtime_dir/pac-port" ]]; then
    printf 'Servidor PAC local nao informou a porta.\n' >&2
    exit 1
fi
read -r pac_port <"$runtime_dir/pac-port"
if [[ ! "$pac_port" =~ ^[0-9]+$ ]]; then
    printf 'Porta PAC local invalida.\n' >&2
    exit 1
fi
pac_url="http://127.0.0.1:$pac_port/discord-tor.pac"
curl --silent --show-error --fail --max-time 2 "$pac_url" --output /dev/null

printf 'Reiniciando Discord Flatpak com PAC local...\n'
flatpak kill "$APP_ID" >/dev/null 2>&1 || true
sleep 1
flatpak run "$APP_ID" "--proxy-pac-url=$pac_url" >>"$state_dir/discord.log" 2>&1 &
discord_pid=$!
discord_started=1

printf 'Ativo. Apenas discord.gg passa pelo Tor; midia e demais trafegos ficam diretos.\n'
printf 'Mantenha este terminal aberto. Ctrl+C encerra o Discord e o Tor.\n'
set +e
wait "$discord_pid"
discord_status=$?
set -e
discord_started=0

if (( discord_status != 0 )); then
    printf 'Discord encerrou com status %d.\n' "$discord_status" >&2
    exit "$discord_status"
fi
printf 'Discord encerrado; finalizando Tor.\n'
