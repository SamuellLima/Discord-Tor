#!/usr/bin/env bash
set -Eeuo pipefail

umask 077

readonly APP_ID='com.discordapp.Discord'
readonly SOCKS_HOST='127.0.0.1'
readonly SOCKS_PORT='19060'
readonly SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd -P)"
readonly BANNER_FILE="$SCRIPT_DIR/internal/banner/art.txt"

log_file=''

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

log_stamp() {
    [[ -n "$log_file" ]] || return 0
    printf '%s\n' "$(date '+%H:%M:%S')" >>"$log_file"
}

log_err() {
    [[ -n "$log_file" ]] || return 0
    printf '%s ERRO %s\n' "$(date '+%H:%M:%S')" "$*" >>"$log_file"
}

fail() {
    say_err 'ERRO' "$*"
    log_err "$*"
    exit 1
}

init_log() {
    local docs=''
    if command -v xdg-user-dir >/dev/null 2>&1; then
        docs="$(xdg-user-dir DOCUMENTS 2>/dev/null || true)"
    fi
    if [[ -z "$docs" || "$docs" == "$HOME" ]]; then
        if [[ -d "${HOME}/Documentos" ]]; then
            docs="${HOME}/Documentos"
        else
            docs="${HOME}/Documents"
        fi
    fi
    local log_dir="${docs}/Discord-Tor"
    mkdir -p -- "$log_dir" || return 0
    log_file="${log_dir}/discord-tor.log"
    say 'DEBUG' "arquivo de log: $log_file"
}

print_banner() {
    printf '\n'
    if [[ -f "$BANNER_FILE" ]]; then
        cat -- "$BANNER_FILE"
        printf '\n'
    fi
    say 'DEBUG' 'launcher Fedora em modo terminal (debug)'
}

port_in_use() {
    local port="$1"
    if command -v ss >/dev/null 2>&1; then
        if [[ -n "$(ss -H -ltn "sport = :${port}" 2>/dev/null || true)" ]]; then
            return 0
        fi
        return 1
    fi
    (echo >/dev/tcp/127.0.0.1/"$port") >/dev/null 2>&1
}

listening_pids() {
    local port="$1"
    if command -v ss >/dev/null 2>&1; then
        ss -H -lptn "sport = :${port}" 2>/dev/null \
            | sed -n 's/.*pid=\([0-9]\+\).*/\1/p' \
            | sort -u || true
        return 0
    fi
    if command -v lsof >/dev/null 2>&1; then
        lsof -t -iTCP:"$port" -sTCP:LISTEN 2>/dev/null || true
    fi
    return 0
}

free_conflicting_tor() {
    local port="$1"
    log_stamp
    if ! port_in_use "$port"; then
        say 'DEBUG' "porta ${port} livre"
        return 0
    fi

    say 'DEBUG' "porta ${port} em uso; encerrando Tor conflitante"
    local pids
    pids="$(listening_pids "$port")"
    if [[ -z "$pids" ]]; then
        say 'DEBUG' "PID da porta ${port} nao encontrado; tentando fuser"
        if command -v fuser >/dev/null 2>&1; then
            fuser -k "${port}/tcp" >/dev/null 2>&1 || true
        fi
    else
        local pid
        for pid in $pids; do
            say 'DEBUG' "encerrando PID ${pid} na porta ${port}"
            kill "$pid" >/dev/null 2>&1 || true
        done
        sleep 0.4
        for pid in $pids; do
            if kill -0 "$pid" >/dev/null 2>&1; then
                say 'DEBUG' "SIGKILL PID ${pid}"
                kill -9 "$pid" >/dev/null 2>&1 || true
            fi
        done
    fi

    local waited=0
    while port_in_use "$port" && (( waited < 80 )); do
        sleep 0.1
        waited=$((waited + 1))
    done
    if port_in_use "$port"; then
        fail "porta ${port} continua em uso apos encerrar o Tor conflitante"
    fi
    say 'OK' "porta ${port} liberada"
}

init_log
log_stamp
print_banner

for dependency in tor flatpak curl python3 flock; do
    say 'DEBUG' "conferindo dependencia: $dependency"
    if ! command -v "$dependency" >/dev/null 2>&1; then
        fail "Dependencia ausente: $dependency"
    fi
done
say 'OK' 'dependencias encontradas'

if [[ ${EUID} -eq 0 ]]; then
    fail 'Nao execute como root.'
fi

say 'DEBUG' "conferindo Flatpak ${APP_ID}"
log_stamp
if ! flatpak info "$APP_ID" >/dev/null 2>&1; then
    fail "Discord Flatpak ($APP_ID) nao esta instalado."
fi
say 'OK' "Flatpak ${APP_ID} instalado"

runtime_parent="${XDG_RUNTIME_DIR:-/tmp}"
runtime_dir="$(mktemp -d -- "$runtime_parent/golive-tor.XXXXXX")"
state_parent="${XDG_STATE_HOME:-${HOME:?HOME nao definido}/.local/state}"
state_dir="$state_parent/golive-tor"
mkdir -p -- "$state_dir/tor-data"
say 'DEBUG' "runtime: $runtime_dir"
say 'DEBUG' "estado: $state_dir"

tor_pid=''
pac_pid=''
discord_started=0
cleanup() {
    trap - EXIT INT TERM HUP
    say 'DEBUG' 'cleanup: encerrando Discord, PAC e Tor'
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

say 'DEBUG' 'adquirindo lock do launcher'
log_stamp
exec 9>>"$state_dir/launcher.lock"
if ! flock -n 9; then
    fail 'Discord-Tor ja esta em execucao.'
fi
say 'OK' 'lock adquirido'

say 'DEBUG' "verificando conflito na porta SOCKS ${SOCKS_PORT}"
free_conflicting_tor "$SOCKS_PORT"

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
say 'DEBUG' "torrc em $runtime_dir/torrc"

say 'DEBUG' "iniciando Tor local em ${SOCKS_HOST}:${SOCKS_PORT}"
log_stamp
tor -f "$runtime_dir/torrc" >>"$state_dir/tor.log" 2>&1 &
tor_pid=$!
say 'OK' "processo Tor iniciado (pid ${tor_pid})"

deadline=$((SECONDS + 300))
tor_ready=0
last_talk=0
attempt=0
say 'DEBUG' 'aguardando SOCKS5 + TLS ate gateway.discord.gg'
log_stamp
while (( SECONDS < deadline )); do
    if ! kill -0 "$tor_pid" 2>/dev/null; then
        fail "Tor encerrou durante a inicializacao. Consulte $state_dir/tor.log"
    fi
    attempt=$((attempt + 1))
    if curl --silent --show-error --max-time 8 --connect-timeout 4 \
        --proxy "socks5h://$SOCKS_HOST:$SOCKS_PORT" https://gateway.discord.gg/ \
        --output /dev/null 2>/dev/null; then
        tor_ready=1
        say 'DEBUG' "prova do gateway ok na tentativa ${attempt}"
        break
    fi
    if (( SECONDS - last_talk >= 5 )); then
        say 'DEBUG' "gateway ainda indisponivel (tentativa ${attempt})"
        last_talk=$SECONDS
    fi
    sleep 1
done

if (( ! tor_ready )); then
    fail 'Tor nao conseguiu acessar o gateway do Discord em 5 minutos.'
fi
say 'OK' 'Tor confirmado: SOCKS5 e TLS do gateway estao validos.'

say 'DEBUG' 'subindo servidor PAC em loopback'
log_stamp
python3 "$SCRIPT_DIR/cmd/fedora/pac_server.py" "$SOCKS_HOST:$SOCKS_PORT" \
    >"$runtime_dir/pac-port" 2> >(tee -a "$state_dir/pac.log" >&2) &
pac_pid=$!
say 'DEBUG' "PAC pid ${pac_pid}"

deadline=$((SECONDS + 10))
while [[ ! -s "$runtime_dir/pac-port" ]] && (( SECONDS < deadline )); do
    if ! kill -0 "$pac_pid" 2>/dev/null; then
        fail "Servidor PAC local nao iniciou. Consulte $state_dir/pac.log"
    fi
    sleep 0.1
done
if [[ ! -s "$runtime_dir/pac-port" ]]; then
    fail 'Servidor PAC local nao informou a porta.'
fi
read -r pac_port <"$runtime_dir/pac-port"
if [[ ! "$pac_port" =~ ^[0-9]+$ ]]; then
    fail 'Porta PAC local invalida.'
fi
pac_url="http://127.0.0.1:$pac_port/discord-tor.pac"
say 'DEBUG' "conferindo PAC em $pac_url"
curl --silent --show-error --fail --max-time 2 "$pac_url" --output /dev/null \
    || fail "PAC local nao respondeu em $pac_url"
say 'OK' "PAC local em $pac_url"

say 'DEBUG' "reiniciando Discord Flatpak ${APP_ID} com PAC local"
log_stamp
flatpak kill "$APP_ID" >/dev/null 2>&1 || true
sleep 1
flatpak run "$APP_ID" \
    "--proxy-pac-url=$pac_url" \
    "--force-webrtc-ip-handling-policy=default_public_interface_only" \
    >>"$state_dir/discord.log" 2>&1 &
discord_pid=$!
discord_started=1
say 'OK' "Discord aberto (pid ${discord_pid}). Apenas discord.gg passa pelo Tor; midia fica direta."
say 'INFO' 'Mantenha este terminal aberto. Ctrl+C encerra o Discord e o Tor.'

set +e
wait "$discord_pid"
discord_status=$?
set -e
discord_started=0

if (( discord_status != 0 )); then
    fail "Discord encerrou com status $discord_status."
fi
say 'INFO' 'Discord encerrado; finalizando Tor.'
log_stamp
