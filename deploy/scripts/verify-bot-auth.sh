#!/bin/sh
set -eu
root=$(CDPATH= cd -- "$(dirname -- "$0")/../.." && pwd)
docker build -f "$root/deploy/bot-auth-verification/Dockerfile" -t meowgram-bot-auth-verification "$root"
docker run --rm --network none --read-only --tmpfs /tmp:rw,noexec,nosuid,size=128m --cap-drop ALL --security-opt no-new-privileges meowgram-bot-auth-verification
