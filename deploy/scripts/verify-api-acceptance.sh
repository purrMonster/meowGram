#!/bin/sh
set -eu
root=$(CDPATH= cd -- "$(dirname -- "$0")/../.." && pwd)
compose="$root/deploy/api-acceptance.compose.yml"
trap 'docker compose -p meowgram-api-acceptance -f "$compose" down --volumes --remove-orphans' EXIT
docker compose -p meowgram-api-acceptance -f "$compose" run --build --rm acceptance
