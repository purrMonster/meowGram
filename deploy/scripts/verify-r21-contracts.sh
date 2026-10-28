#!/bin/sh
set -eu
repo=$(CDPATH= cd -- "$(dirname -- "$0")/../.." && pwd)
temp_root=$(CDPATH= cd -- "${TMPDIR:-/tmp}" && pwd -P)
scratch=$(mktemp -d "$temp_root/meowgram-r21-contract-XXXXXXXX")
project="meowgram-r21-contract-$$"
compose="$repo/deploy/r21-contracts.compose.yml"
cleanup() {
 status=$?
 trap - 0 HUP INT TERM
 docker compose -p "$project" -f "$compose" down --volumes --remove-orphans || status=1
 if [ "$(dirname -- "$scratch")" != "$temp_root" ]; then
  echo 'Unsafe cleanup path.' >&2
  exit 1
 fi
 case "$(basename -- "$scratch")" in
  meowgram-r21-contract-????????) rm -rf -- "$scratch" || status=1 ;;
  *) echo 'Unsafe cleanup path.' >&2; exit 1 ;;
 esac
 exit "$status"
}
trap cleanup 0
trap 'exit 129' HUP
trap 'exit 130' INT
trap 'exit 143' TERM
mkdir "$scratch/r2"
git -C "$repo" archive --format=tar --output="$scratch/r2.tar" v2.0.0 server
tar -xf "$scratch/r2.tar" -C "$scratch/r2"
docker build --build-context "r2=$scratch/r2" -f "$repo/deploy/r21-contracts/Dockerfile" -t "$project-contracts" "$repo"
for zone in UTC Asia/Kolkata; do
 echo "R2.1 contract TZ=$zone"
 docker compose -p "$project" -f "$compose" run --rm -e "TZ=$zone" contracts
 # Fresh actual-v2 database for each timezone.
 docker compose -p "$project" -f "$compose" down --volumes --remove-orphans
done
