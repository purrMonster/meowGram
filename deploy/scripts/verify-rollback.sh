#!/bin/sh
set -eu
repo=$(CDPATH= cd -- "$(dirname -- "$0")/../.." && pwd)
scratch=$(mktemp -d)
cleanup() {
 docker compose -p meowgram-rollback -f "$repo/deploy/rollback.compose.yml" down --volumes --remove-orphans
 rm -rf -- "$scratch"
}
trap cleanup EXIT HUP INT TERM
git -C "$repo" archive --format=tar --output="$scratch/previous.tar" v1.1.0 server
tar -xf "$scratch/previous.tar" -C "$scratch"
docker build --build-context "previous=$scratch" -f "$repo/deploy/rollback/Dockerfile" -t meowgram-rollback:verification "$repo"
docker compose -p meowgram-rollback -f "$repo/deploy/rollback.compose.yml" run --rm rollback
