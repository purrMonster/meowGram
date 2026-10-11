#!/bin/sh
set -eu
case "${1:-}" in
  ''|--android) ;;
  *) echo 'Usage: verify.sh [--android]' >&2; exit 2 ;;
esac
root=$(CDPATH= cd -- "$(dirname -- "$0")/../.." && pwd)
project="meowgram-verify-$(date +%s)-$$"
compose() { docker compose -p "$project" -f "$root/deploy/verify.compose.yml" "$@"; }
trap 'compose down --volumes --remove-orphans' EXIT
compose run --rm server
compose run --rm restore
compose run --build --rm client
compose run --build --rm web-smoke
if [ "${1:-}" = --android ]; then compose run --build --rm android; fi
