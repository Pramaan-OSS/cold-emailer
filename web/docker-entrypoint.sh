#!/bin/sh
# Render the runtime config from container env so a single built image serves
# any deployment. Runs before nginx starts (nginx /docker-entrypoint.d hook).
#
# COLD_EMAIL_CONFIG_OUT moves where it writes, which is how a static host that
# has no container start renders the same file at build time. One definition
# of the key set, so the two paths cannot drift apart.
set -eu

CONFIG_OUT="${COLD_EMAIL_CONFIG_OUT:-/usr/share/nginx/html/config.js}"
mkdir -p "$(dirname "$CONFIG_OUT")"

# Values are written into JavaScript string literals, so a double quote, a
# backslash or a line break in one would end the literal early and take the
# whole config with it, leaving the app with no API_URL at all. Escape rather
# than trust whatever ended up in .env.
js() {
    printf '%s' "$1" | sed -e 's/\\/\\\\/g' -e 's/"/\\"/g' | tr -d '\r\n'
}

cat > "$CONFIG_OUT" <<EOF
window.__COLD_EMAIL_ENV__ = {
  API_URL: "$(js "${COLD_EMAIL_API_URL:-}")",
  APP_URL: "$(js "${COLD_EMAIL_APP_URL:-}")",
  BETA_NOTICE: "$(js "${COLD_EMAIL_BETA_NOTICE:-}")",
  COMPANY_LOGOS: "$(js "${COLD_EMAIL_COMPANY_LOGOS:-}")"
};
EOF

# The redirect truncates in place and keeps whatever mode the built file had, so
# a restrictive umask or checkout leaves nginx serving 403 for the whole config.
chmod 644 "$CONFIG_OUT"

# The build hashes the fixed inline bootstrap; deployment supplies resource origins.
CSP_DIR=$(dirname "$CONFIG_OUT")
if [ -f "$CSP_DIR/csp-template.txt" ]; then
    origin() {
        [ -n "$1" ] || return 0
        case "$1" in /*) return 0 ;; esac
        scheme=${1%%://*}
        case "$scheme" in http|https|ws|wss) ;; *) printf 'Invalid CSP resource URL\n' >&2; exit 1 ;; esac
        authority=${1#*://}
        authority=${authority%%/*}
        authority=${authority%%\?*}
        authority=${authority%%\#*}
        authority=${authority##*@}
        printf '%s' "$authority" | grep -Eq '^([a-zA-Z0-9._-]+|\[[a-fA-F0-9:]+\])(:[0-9]+)?$' || {
            printf 'Invalid CSP resource origin\n' >&2; exit 1;
        }
        printf '%s://%s' "$scheme" "$authority"
    }
    defaults() { if [ -f "$CSP_DIR/csp-origins.txt" ]; then sed -n "${1}p" "$CSP_DIR/csp-origins.txt"; fi; }
    api_url=${COLD_EMAIL_API_URL:-${VITE_API_URL:-$(defaults 1)}}
    api=$(origin "$api_url")
    connections=""
    for resource in ${COLD_EMAIL_CSP_CONNECT_ORIGINS:-${VITE_CSP_CONNECT_ORIGINS:-$(defaults 4)}} ${PRAMAAN_MANAGED_AUTH_URL:-}; do
        connections="$connections $(origin "$resource")"
    done
    # Discover deployment configuration before serving an immutable CSP header.
    realtime=${WEBSOCKET_URL:-}
    if [ -z "$realtime" ] && [ -n "$api" ]; then
        config_url="${api_url%/}"
        config_url="${config_url%/v1}/v1/auth/config"
        if command -v curl >/dev/null 2>&1 && command -v jq >/dev/null 2>&1 &&
            deployment=$(curl --fail --silent --proto '=http,https' --connect-timeout 2 --max-time 4 --max-filesize 65536 --retry 2 --retry-connrefused --retry-delay 1 --retry-max-time 10 "$config_url") &&
            realtime=$(printf '%s' "$deployment" | jq -er 'if .websocket_url == null then "" elif (.websocket_url | type) == "string" then .websocket_url else error("invalid websocket_url") end'); then
            :
        else
            realtime=""
        printf 'Realtime CSP discovery unavailable; the dashboard can use the API-origin socket route. Explicit CSP origins are needed only for direct gateway connections.\n' >&2
        fi
    fi
    if [ -n "$realtime" ]; then
        if realtime_origin=$(origin "$realtime") && [ -n "$realtime_origin" ]; then
            api_websocket=$(printf '%s' "$api" | sed 's/^http/ws/')
            case " $api $api_websocket $connections " in
                *" $realtime_origin "*) ;;
                *) connections="$connections $realtime_origin" ;;
            esac
        else
            printf 'Ignoring invalid discovered realtime origin; CSP remains restricted.\n' >&2
        fi
    fi
    websocket=$(printf '%s' "$api" | sed 's/^http/ws/')
    policy=$(sed -e "s#__API_SOURCES__#$api $websocket#g" -e "s#__API_IMAGE_SOURCE__#$api#g" -e "s#__CONNECT_SOURCES__#$connections#g" "$CSP_DIR/csp-template.txt")
    if [ "${#policy}" -gt 1973 ]; then printf 'Dashboard CSP exceeds the static host header limit\n' >&2; exit 1; fi
    printf '%s\n' "$policy" > "$CSP_DIR/csp-policy.txt"
    if [ -f "$CSP_DIR/_headers" ]; then
        CSP_POLICY="$policy" awk '/^  Content-Security-Policy:/ && !replaced { print "  Content-Security-Policy: " ENVIRON["CSP_POLICY"]; replaced=1; next } { print }' "$CSP_DIR/_headers" > "$CSP_DIR/_headers.new"
        mv "$CSP_DIR/_headers.new" "$CSP_DIR/_headers"
    fi
    if [ "$CONFIG_OUT" = /usr/share/nginx/html/config.js ]; then
        CSP_POLICY="$policy" awk '/^add_header Content-Security-Policy/ { print "add_header Content-Security-Policy \"" ENVIRON["CSP_POLICY"] "\" always;"; next } { print }' /etc/nginx/coldemail-security-headers.conf > /etc/nginx/coldemail-security-headers.conf.new
        mv /etc/nginx/coldemail-security-headers.conf.new /etc/nginx/coldemail-security-headers.conf
    fi
    chmod 644 "$CSP_DIR/csp-policy.txt"
fi
