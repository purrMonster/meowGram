#!/usr/bin/env bash
# ==============================================================================
# meowGram Production Build Pipeline (Bash / CI/CD)
# Release 1 Packaging Sprint
# ==============================================================================

set -euo pipefail

TARGET="${1:-all}"
APP_DOMAIN="${APP_DOMAIN:-meow.example.home.arpa}"
AUTHELIA_DOMAIN="${AUTHELIA_DOMAIN:-auth.example.home.arpa}"
AUTHELIA_ISSUER="${AUTHELIA_ISSUER:-https://auth.example.home.arpa}"
AUTHELIA_CLIENT_ID="${AUTHELIA_CLIENT_ID:-meowgram}"
VERSION="1.0.1+2"

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
CLIENT_DIR="$PROJECT_ROOT/client"
SERVER_DIR="$PROJECT_ROOT/server"

echo "=========================================================="
echo "  meowGram Production Build Pipeline: Release 1.0.1"
echo "  Target Domain: $APP_DOMAIN"
echo "  Authelia Domain: $AUTHELIA_DOMAIN"
echo "  Target Environment: production"
echo "=========================================================="

# Real values live in the gitignored client/config/production.local.json
# (AGENTS.md: no real domain in any tracked file). The tracked production.json
# holds placeholders only.
if [ -f "$CLIENT_DIR/config/production.local.json" ]; then
  echo "Using configuration file: client/config/production.local.json"
  DART_DEFINES=(
    "--dart-define-from-file=config/production.local.json"
  )
elif [ "$APP_DOMAIN" = "meow.example.home.arpa" ]; then
  echo "Error: create client/config/production.local.json (copy production.json and fill in real values)" >&2
  echo "       or export APP_DOMAIN / AUTHELIA_DOMAIN / AUTHELIA_ISSUER." >&2
  exit 1
else
  DART_DEFINES=(
    "--dart-define=APP_ENV=production"
    "--dart-define=APP_DOMAIN=$APP_DOMAIN"
    "--dart-define=HTTP_PORT=443"
    "--dart-define=USE_SECURE_SCHEMES=true"
    "--dart-define=AUTHELIA_DOMAIN=$AUTHELIA_DOMAIN"
    "--dart-define=AUTHELIA_ISSUER_URL=$AUTHELIA_ISSUER"
    "--dart-define=AUTHELIA_CLIENT_ID=$AUTHELIA_CLIENT_ID"
    "--dart-define=API_BASE_URL=https://$APP_DOMAIN"
    "--dart-define=WS_BASE_URL=wss://$APP_DOMAIN/ws"
    "--dart-define=WS_TICKET_ENDPOINT=/api/ws-ticket"
    "--dart-define=SYNC_ENDPOINT=/api/messages/sync"
    "--dart-define=HEALTH_ENDPOINT=/healthz"
  )
fi

cd "$CLIENT_DIR"

echo ""
echo "[1/7] Fetching Flutter dependencies..."
flutter pub get

# Target: Web
if [[ "$TARGET" == "all" || "$TARGET" == "web" ]]; then
  echo ""
  echo "[2/7] Building Web Release Bundle..."
  flutter build web --release "${DART_DEFINES[@]}"
  echo "✓ Web bundle compiled to client/build/web"
fi

# Target: Android APK & AAB
if [[ "$TARGET" == "all" || "$TARGET" == "android" ]]; then
  echo ""
  echo "[3/7] Building Android App Bundle (AAB for Google Play)..."
  flutter build appbundle --release "${DART_DEFINES[@]}"

  echo ""
  echo "[4/7] Building Android Universal APK (Direct Sideload)..."
  flutter build apk --release "${DART_DEFINES[@]}"
  echo "✓ Android APK compiled to client/build/app/outputs/flutter-apk/app-release.apk"
fi

# Target: Windows Desktop
if [[ "$TARGET" == "all" || "$TARGET" == "windows" ]]; then
  echo ""
  echo "[5/7] Building Windows Desktop Release..."
  if [[ "$(uname -s)" == *"MINGW"* || "$(uname -s)" == *"CYGWIN"* || "$(uname -s)" == *"MSYS"* ]]; then
    flutter build windows --release "${DART_DEFINES[@]}"
    echo "✓ Windows release compiled to client/build/windows/x64/runner/Release"
  else
    echo "Notice: Windows builds must be executed on a Windows host with Visual Studio C++ workload."
  fi
fi

# Target: macOS Desktop
if [[ "$TARGET" == "all" || "$TARGET" == "macos" ]]; then
  echo ""
  echo "[6/7] Building macOS Desktop Release..."
  if [[ "$(uname)" == "Darwin" ]]; then
    flutter build macos --release "${DART_DEFINES[@]}"
    echo "✓ macOS release compiled to client/build/macos/Build/Products/Release"
  else
    echo "Notice: macOS builds must be executed on a macOS host with Xcode installed."
  fi
fi

# Target: iOS
if [[ "$TARGET" == "all" || "$TARGET" == "ios" ]]; then
  echo ""
  echo "[7/7] Building iOS Archive / IPA..."
  if [[ "$(uname)" == "Darwin" ]]; then
    flutter build ipa --release --no-codesign "${DART_DEFINES[@]}"
    echo "✓ iOS archive compiled to client/build/ios/archive/Runner.xcarchive"
  else
    echo "Notice: iOS IPA builds must be executed on a macOS host with Xcode installed."
  fi
fi

# Backend Docker Image Build
if [[ "$TARGET" == "all" || "$TARGET" == "server" || "$TARGET" == "docker" ]]; then
  cd "$PROJECT_ROOT"
  echo ""
  echo "[*] Building Backend Production Docker Image (meowgram:1.0.1)..."
  docker build -t meowgram:1.0.1 -t meowgram:latest -f "$SERVER_DIR/Dockerfile" "$SERVER_DIR"
  echo "✓ Docker image meowgram:1.0.1 built successfully."
fi

echo ""
echo "=========================================================="
echo "  meowGram Production Packaging Completed Successfully!"
echo "=========================================================="
