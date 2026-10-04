#!/usr/bin/env bash
# ==============================================================================
# meowGram iOS Physical Device Runner Script
# Launches meowGram on connected physical iOS devices using --dart-define-from-file
# with optional cache invalidation (flutter clean & pub get) for Info.plist refresh.
# ==============================================================================

set -euo pipefail

ENV="production"
MODE="release"
DEVICE_ID=""
CLEAN="${CLEAN:-false}"

POSITIONAL_ARGS=()

while [[ $# -gt 0 ]]; do
  case "$1" in
    --clean|-c)
      CLEAN=true
      shift
      ;;
    -d|--device)
      DEVICE_ID="$2"
      shift 2
      ;;
    -h|--help)
      echo "Usage: $0 [ENV] [MODE] [--clean|-c] [-d DEVICE_ID]"
      echo ""
      echo "Arguments:"
      echo "  ENV         Environment to run: production (default) or development"
      echo "  MODE        Flutter run mode: release (default), profile, or debug"
      echo "  --clean, -c Invalidate Flutter/Xcode build cache (runs flutter clean && flutter pub get)"
      echo "  -d, --device Physical iOS device ID or name (default: ios)"
      echo "  -h, --help  Show this help message"
      exit 0
      ;;
    *)
      POSITIONAL_ARGS+=("$1")
      shift
      ;;
  esac
done

if [ ${#POSITIONAL_ARGS[@]} -ge 1 ]; then
  ENV="${POSITIONAL_ARGS[0]}"
fi
if [ ${#POSITIONAL_ARGS[@]} -ge 2 ]; then
  MODE="${POSITIONAL_ARGS[1]}"
fi

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
CLIENT_DIR="$PROJECT_ROOT/client"

CONFIG_FILE="$CLIENT_DIR/config/${ENV}.json"

if [ ! -f "$CONFIG_FILE" ]; then
  echo "Error: Configuration file not found: $CONFIG_FILE"
  echo "Available configurations: development, production"
  exit 1
fi

echo "=========================================================="
echo "  meowGram iOS Physical Device Launch"
echo "  Environment: $ENV"
echo "  Mode:        $MODE"
echo "  Config:      $CONFIG_FILE"
echo "  Clean Build: $CLEAN"
if [ -n "$DEVICE_ID" ]; then
  echo "  Device:      $DEVICE_ID"
else
  echo "  Device:      Auto-detect / ios"
fi
echo "=========================================================="

cd "$CLIENT_DIR"

# Build Cache Invalidation (ensures Xcode picks up updated Info.plist URL schemes)
if [ "$CLEAN" = "true" ] || [ "$CLEAN" = "1" ]; then
  echo ""
  echo "[1/2] Invalidating Flutter & Xcode build cache..."
  flutter clean
  echo ""
  echo "[2/2] Resolving Flutter dependencies..."
  flutter pub get
  echo "✓ Cache cleared and dependencies refreshed."
fi

# Build execution arguments
RUN_ARGS=()

if [ -n "$DEVICE_ID" ]; then
  RUN_ARGS+=("-d" "$DEVICE_ID")
else
  RUN_ARGS+=("-d" "ios")
fi

if [ "$MODE" = "release" ]; then
  RUN_ARGS+=("--release")
elif [ "$MODE" = "debug" ]; then
  RUN_ARGS+=("--debug")
elif [ "$MODE" = "profile" ]; then
  RUN_ARGS+=("--profile")
fi

RUN_ARGS+=("--dart-define-from-file=config/${ENV}.json")

echo ""
echo "Executing: flutter run ${RUN_ARGS[*]}"
flutter run "${RUN_ARGS[@]}"
