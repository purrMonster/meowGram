#!/bin/sh
set -eu
root=$(CDPATH= cd -- "$(dirname -- "$0")/../.." && pwd)
compose="$root/deploy/r21-contracts.compose.yml"
trap 'docker compose -p meowgram-r21-contract -f "$compose" down --volumes --remove-orphans' EXIT
docker compose -p meowgram-r21-contract -f "$compose" build contracts
for zone in UTC Asia/Kolkata; do
  echo "R2.1 contract TZ=$zone"
  docker compose -p meowgram-r21-contract -f "$compose" run --rm -e "TZ=$zone" contracts
done
