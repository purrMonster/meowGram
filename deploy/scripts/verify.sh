#!/bin/sh
set -eu
root=$(CDPATH= cd -- "$(dirname -- "$0")/../.." && pwd)
project="meowgram-verify-$(date +%s)-$$"
compose() { docker compose -p "$project" -f "$root/deploy/verify.compose.yml" "$@"; }
trap 'compose down --volumes --remove-orphans' EXIT
compose run --rm server
compose run --rm restore
compose run --build --rm client
