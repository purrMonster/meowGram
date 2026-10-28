#!/bin/sh
set -eu
repo=$(CDPATH= cd -- "$(dirname -- "$0")/../.." && pwd)
temp_root=$(CDPATH= cd -- "${TMPDIR:-/tmp}" && pwd -P)
scratch=$(mktemp -d "$temp_root/meowgram-rollback-XXXXXXXX")
project="meowgram-rollback-$$"
cleanup() {
 status=$?
 trap - 0 HUP INT TERM
 docker compose -p "$project" -f "$repo/deploy/rollback.compose.yml" down --volumes --remove-orphans || status=1
 # Only this invocation's absolute temporary child may be recursively removed.
 if [ "$(dirname -- "$scratch")" != "$temp_root" ]; then
  echo 'Unsafe cleanup path.' >&2
  exit 1
 fi
 case "$(basename -- "$scratch")" in
  meowgram-rollback-????????) rm -rf -- "$scratch" || status=1 ;;
  *) echo 'Unsafe cleanup path.' >&2; exit 1 ;;
 esac
 exit "$status"
}
trap cleanup 0
trap 'exit 129' HUP
trap 'exit 130' INT
trap 'exit 143' TERM
mkdir "$scratch/previous" "$scratch/r2"
git -C "$repo" archive --format=tar --output="$scratch/previous.tar" v1.1.0 server
tar -xf "$scratch/previous.tar" -C "$scratch/previous"
git -C "$repo" archive --format=tar --output="$scratch/r2.tar" v2.0.0 server
tar -xf "$scratch/r2.tar" -C "$scratch/r2"
docker build --build-context "previous=$scratch/previous" --build-context "r2=$scratch/r2" -f "$repo/deploy/rollback/Dockerfile" -t meowgram-rollback:verification "$repo"
docker compose -p "$project" -f "$repo/deploy/rollback.compose.yml" run --rm rollback
