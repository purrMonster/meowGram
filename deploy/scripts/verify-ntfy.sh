#!/bin/sh
set -eu
root=$(CDPATH= cd -- "$(dirname -- "$0")/../.." && pwd)
docker build -f "$root/deploy/ntfy/Dockerfile" -t meowgram-ntfy-contract:local "$root"
for zone in UTC Asia/Kolkata; do
  docker run --rm --network none --read-only --tmpfs /tmp -e "TZ=$zone" meowgram-ntfy-contract:local
done
