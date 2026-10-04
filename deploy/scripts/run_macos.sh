#!/usr/bin/env bash
# ==============================================================================
# meowGram macOS Runner Script
# Launches meowGram on macOS using --dart-define-from-file to eliminate shell escaping
# ==============================================================================

set -euo pipefail

ENV="${1:-production}"
MODE="${2:-release}"

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
echo "  meowGram macOS Launch: $ENV (mode: $MODE)"
echo "  Config: $CONFIG_FILE"
echo "=========================================================="

cd "$CLIENT_DIR"

if [ "$MODE" = "release" ]; then
  flutter run -d macos --release --dart-define-from-file="config/${ENV}.json"
else
  flutter run -d macos --dart-define-from-file="config/${ENV}.json"
fi
