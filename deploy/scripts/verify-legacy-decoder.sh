#!/bin/sh
set -eu
root=$(CDPATH= cd -- "$(dirname -- "$0")/../.." && pwd)
source=$(git -C "$root" rev-parse 'v2.0.0^{}')
test "$source" = be15cec6ceed2cb4a21b69cfe5e4548d2bfd818c
scratch=$(mktemp -d)
image="meowgram-legacy-decoder:$(basename "$scratch" | tr '[:upper:]' '[:lower:]')"
cleanup() {
 docker image rm "$image" >/dev/null 2>&1 || true
 rm -rf -- "$scratch"
}
trap cleanup EXIT INT TERM
git -C "$root" archive --format=tar --output="$scratch/source.tar" "$source"
tar -xf "$scratch/source.tar" -C "$scratch"
printf 'Actual legacy source: %s\n' "$source"
docker build --build-context "legacy=$scratch" --build-arg "LEGACY_COMMIT=$source" -f "$root/deploy/legacy-decoder/Dockerfile" -t "$image" "$root"
for zone in UTC Asia/Kolkata; do
 printf 'Legacy decoder TZ=%s\n' "$zone"
 docker run --rm --network none --read-only --tmpfs /tmp:rw,noexec,nosuid,size=16m --cap-drop ALL --security-opt no-new-privileges -e "TZ=$zone" "$image"
done
docker image inspect "$image" --format '{{.Id}}'

