#!/bin/sh
set -eu
test "$(id -u)" -ne 0
nginx -g 'daemon off;' &
nginx_pid=$!
trap 'kill "$nginx_pid"; wait "$nginx_pid" || true' EXIT
attempt=0
until wget -q -O /tmp/home.html http://127.0.0.1:8080/; do
  attempt=$((attempt + 1))
  test "$attempt" -lt 20
  sleep 1
done
wget -S -O /tmp/route.html http://127.0.0.1:8080/scratch/callback 2>/tmp/headers
cmp /tmp/home.html /tmp/route.html
cmp /tmp/home.html /usr/share/nginx/html/index.html
grep -qi 'Cache-Control: no-store' /tmp/headers
wget -q -O /tmp/bootstrap.js http://127.0.0.1:8080/flutter_bootstrap.js
test -s /tmp/bootstrap.js
echo 'Web runtime passed: non-root, SPA fallback, no-store HTML and bootstrap asset.'
