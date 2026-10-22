#!/bin/sh
set -eu
context=$(CDPATH= cd -- "$(dirname -- "$0")/../directory-verification" && pwd)
image=meowgram-directory-verification
docker build -t "$image" "$context"
docker run --rm --network none --read-only \
  --tmpfs /tmp:rw,noexec,nosuid,size=128m --cap-drop ALL \
  --security-opt no-new-privileges "$image"
