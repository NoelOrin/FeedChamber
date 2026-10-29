#!/usr/bin/env bash
# Guard: the metrics listener must never be reachable from outside the
# container network.
#
# METRICS_PORT (default 9091) serves /metrics with no authentication. It is
# safe only because it is never published to the host: compose uses `expose`
# (container-network only) and nginx has no upstream pointing at it. A single
# edit from `expose:` to `ports:` would put DB pool sizes and traffic internals
# on the public internet, and nothing else in the repo would notice.
#
# This script is the thing that notices. Run it in CI and locally.
#
# Usage: scripts/check-metrics-exposure.sh [compose-file ...]
#   With no arguments it checks every deploy/docker-compose*.yml.
#
# Exit codes: 0 = clean, 1 = violation found.

set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$repo_root"

if [ "$#" -gt 0 ]; then
    files=("$@")
else
    shopt -s nullglob
    files=(deploy/docker-compose*.yml)
    shopt -u nullglob
fi

if [ "${#files[@]}" -eq 0 ]; then
    echo "no compose files found; nothing to check"
    exit 0
fi

fail=0

for f in "${files[@]}"; do
    if [ ! -f "$f" ]; then
        echo "FAIL $f: file not found"
        fail=1
        continue
    fi

    # Parse per service with a tiny awk state machine rather than grepping the
    # whole file: a `ports:` block belonging to postgres must not be attributed
    # to backend, and vice versa.
    violations="$(
        awk '
            # Track the service whose block we are inside (2-space indent keys).
            /^  [A-Za-z0-9_.-]+:[[:space:]]*$/ {
                service = $1
                sub(/:$/, "", service)
                in_ports = 0
                next
            }
            # A `ports:` key opens a published-port list for this service.
            /^    ports:[[:space:]]*$/ {
                in_ports = 1
                next
            }
            # Any other 4-space-indented key ends the ports block.
            /^    [A-Za-z0-9_.-]+:/ {
                in_ports = 0
            }
            # Inside a ports block, look for the metrics port in either the
            # short syntax ("9091:9091", "127.0.0.1:9091:9091") or long syntax
            # (target: 9091 / published: 9091).
            in_ports && /9091/ {
                printf "%s:%s\n", service, $0
            }
        ' "$f"
    )"

    if [ -n "$violations" ]; then
        echo "FAIL $f: METRICS_PORT (9091) is published to the host:"
        while IFS= read -r line; do
            printf '    %s\n' "$line"
        done <<<"$violations"
        echo "    The metrics endpoint is unauthenticated. Use 'expose' instead of 'ports',"
        echo "    or bind it to an internal interface only."
        fail=1
    else
        echo "OK   $f: 9091 not published"
    fi
done

# Second path to the same leak: proxying the metrics port from nginx. The
# requirement is that /metrics never appears in an nginx upstream, so any
# reference to 9091 in the proxy config is a violation regardless of path.
nginx_conf="deploy/nginx/nginx.conf"
if [ -f "$nginx_conf" ]; then
    nginx_hits="$(grep -n '9091' "$nginx_conf" || true)"
    if [ -n "$nginx_hits" ]; then
        echo "FAIL $nginx_conf: nginx references the metrics port:"
        while IFS= read -r line; do
            printf '    %s\n' "$line"
        done <<<"$nginx_hits"
        echo "    nginx MUST NOT proxy METRICS_PORT; scrape it over the container network."
        fail=1
    else
        echo "OK   $nginx_conf: metrics port not proxied"
    fi
fi

exit "$fail"
