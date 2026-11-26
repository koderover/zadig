#!/bin/sh
set -eu

fail() { echo "$*" >&2; exit 1; }
server_url= token= download_url= workspace= upgrade=false
while [ "$#" -gt 0 ]; do
    case "$1" in
        --upgrade) upgrade=true; shift; continue ;;
        --server-url|--token|--download-url|--work-dir)
            [ "$#" -ge 2 ] || fail "Missing value for $1"
            case "$1" in
                --server-url) server_url=$2 ;;
                --token) token=$2 ;;
                --download-url) download_url=$2 ;;
                --work-dir) workspace=$2 ;;
            esac
            shift 2 ;;
        *) fail "Unknown argument: $1" ;;
    esac
done
[ -n "$server_url" ] && [ -n "$token" ] && [ -n "$download_url" ] || fail 'server-url, token and download-url are required'
for url in "$server_url" "$download_url"; do
    case "$url" in http://?*|https://?*) ;; *) fail 'Installation requires an HTTP(S) URL' ;; esac
    case "$url" in *\?*|*\#*|*[[:space:]]*) fail 'URLs must not contain whitespace, query or fragment' ;; esac
    authority=${url#*://}; authority=${authority%%/*}
    case "$authority" in ''|*@*) fail 'URL host is missing or contains credentials' ;; esac
done
command -v curl >/dev/null 2>&1 || fail 'curl is required'
run_as_root() {
    if [ "$(id -u)" -eq 0 ]; then "$@"; else sudo -H "$@"; fi
}
if [ "$(id -u)" -ne 0 ]; then
    command -v sudo >/dev/null 2>&1 || fail 'Run this installer as root or install sudo'
    sudo -H true
fi
agent=/usr/local/bin/zadig-agent
if [ -e "$agent" ] || [ -L "$agent" ]; then
    "$upgrade" || fail 'Agent already exists; use the recovery or upgrade command'
    [ -f "$agent" ] && [ ! -L "$agent" ] || fail 'Refusing to replace a non-regular agent file'
fi
agent_file=$(mktemp)
agent_stage=
cleanup() {
    rm -f "$agent_file"
    if [ -n "$agent_stage" ]; then run_as_root rm -f "$agent_stage"; fi
}
trap cleanup EXIT
protocols='=https'
case "$download_url" in http://*) protocols='=http,https' ;; esac
curl --proto "$protocols" --proto-redir "$protocols" -fsSL "$download_url" -o "$agent_file"
if "$upgrade" && { [ -e "$agent" ] || [ -L "$agent" ]; }; then
    run_as_root "$agent" stop
    sleep 6
    chmod 700 "$agent_file"
    run_as_root "$agent_file" wait-for-stop
    run_as_root cp -p "$agent" "$agent.previous-$(date +%s)-$$"
fi
run_as_root mkdir -p /usr/local/bin
agent_stage=$(run_as_root mktemp /usr/local/bin/.zadig-agent-install-XXXXXX)
run_as_root install -m 755 "$agent_file" "$agent_stage"
run_as_root mv -f "$agent_stage" "$agent"
agent_stage=
set -- install --server-url "$server_url" --token "$token"
if [ -n "$workspace" ]; then set -- "$@" --work-dir "$workspace"; fi
run_as_root "$agent" "$@"
