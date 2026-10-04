# ==============================================================================
# meowGram Production Build Pipeline (PowerShell)
# Release 1 Packaging Sprint
# ==============================================================================
param (
    [string]$Target = "all",
    [string]$AppDomain = "meowgram.purrbrews.cc",
    [string]$AutheliaDomain = "auth.purrbrews.cc",
    [string]$AutheliaIssuer = "https://auth.purrbrews.cc",
    [string]$AutheliaClientId = "meowgram-client",
    [string]$Version = "1.0.0+1"
)

$ScriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$ProjectRoot = (Resolve-Path "$ScriptDir\..\..").Path
$ClientDir = "$ProjectRoot\client"
$ServerDir = "$ProjectRoot\server"

# Check OS Platform
$IsMac = [System.Runtime.InteropServices.RuntimeInformation]::IsOSPlatform([System.Runtime.InteropServices.OSPlatform]::OSX)
$IsWin = [System.Runtime.InteropServices.RuntimeInformation]::IsOSPlatform([System.Runtime.InteropServices.OSPlatform]::Windows)

# Locate Flutter executable
$FlutterBin = "flutter"
if (-not (Get-Command flutter -ErrorAction SilentlyContinue)) {
    if (Test-Path "C:\Users\jyotirmoyc\Documents\Tools\flutter\bin\flutter.bat") {
        $FlutterBin = "C:\Users\jyotirmoyc\Documents\Tools\flutter\bin\flutter.bat"
    }
}

Write-Host "==========================================================" -ForegroundColor Cyan
Write-Host "  meowGram Production Build Pipeline: Release 1.0.0" -ForegroundColor Cyan
Write-Host "  Target Domain: $AppDomain" -ForegroundColor Cyan
Write-Host "  Authelia Domain: $AutheliaDomain" -ForegroundColor Cyan
Write-Host "  Target Environment: production" -ForegroundColor Cyan
Write-Host "==========================================================" -ForegroundColor Cyan

# Production --dart-define parameters
$ConfigFile = "$ClientDir\config\production.json"
if (Test-Path $ConfigFile) {
    Write-Host "Using configuration file: client/config/production.json" -ForegroundColor Cyan
    $DartDefines = @("--dart-define-from-file=config/production.json")
} else {
    $DartDefines = @(
        "--dart-define=APP_ENV=production",
        "--dart-define=APP_DOMAIN=$AppDomain",
        "--dart-define=HTTP_PORT=443",
        "--dart-define=USE_SECURE_SCHEMES=true",
        "--dart-define=AUTHELIA_DOMAIN=$AutheliaDomain",
        "--dart-define=AUTHELIA_ISSUER_URL=$AutheliaIssuer",
        "--dart-define=AUTHELIA_CLIENT_ID=$AutheliaClientId",
        "--dart-define=API_BASE_URL=https://$AppDomain",
        "--dart-define=WS_BASE_URL=wss://$AppDomain/ws",
        "--dart-define=SYNC_ENDPOINT=/api/messages/sync",
        "--dart-define=HEALTH_ENDPOINT=/healthz"
    )
}

Set-Location $ClientDir

Write-Host "`n[1/7] Fetching Flutter dependencies..." -ForegroundColor Yellow
& $FlutterBin pub get

# Target: Web
if ($Target -eq "all" -or $Target -eq "web") {
    Write-Host "`n[2/7] Building Web Release Bundle..." -ForegroundColor Green
    & $FlutterBin build web --release $DartDefines
    Write-Host "[OK] Web bundle compiled to client/build/web" -ForegroundColor Green
}

# Target: Android APK & AAB
if ($Target -eq "all" -or $Target -eq "android") {
    Write-Host "`n[3/7] Building Android App Bundle (AAB for Google Play)..." -ForegroundColor Green
    & $FlutterBin build appbundle --release $DartDefines

    Write-Host "`n[4/7] Building Android Universal APK (Direct Sideload)..." -ForegroundColor Green
    & $FlutterBin build apk --release $DartDefines
    Write-Host "[OK] Android APK compiled to client/build/app/outputs/flutter-apk/app-release.apk" -ForegroundColor Green
}

# Target: Windows Desktop
if ($Target -eq "all" -or $Target -eq "windows") {
    Write-Host "`n[5/7] Building Windows Desktop Release..." -ForegroundColor Green
    if ($IsWin) {
        & $FlutterBin build windows --release $DartDefines
        Write-Host "[OK] Windows release compiled to client/build/windows/x64/runner/Release" -ForegroundColor Green
    } else {
        Write-Host "Notice: Windows desktop builds must be executed on a Windows host with Visual Studio C++ workload." -ForegroundColor DarkYellow
    }
}

# Target: macOS Desktop
if ($Target -eq "all" -or $Target -eq "macos") {
    Write-Host "`n[6/7] Building macOS Desktop Release..." -ForegroundColor Green
    if ($IsMac) {
        & $FlutterBin build macos --release $DartDefines
        Write-Host "[OK] macOS release compiled to client/build/macos/Build/Products/Release" -ForegroundColor Green
    } else {
        Write-Host "Notice: macOS builds must be executed on a macOS host with Xcode installed." -ForegroundColor DarkYellow
    }
}

# Target: iOS
if ($Target -eq "all" -or $Target -eq "ios") {
    Write-Host "`n[7/7] Building iOS Archive / IPA..." -ForegroundColor Green
    if ($IsMac) {
        & $FlutterBin build ipa --release --no-codesign $DartDefines
        Write-Host "[OK] iOS archive compiled to client/build/ios/archive/Runner.xcarchive" -ForegroundColor Green
    } else {
        Write-Host "Notice: iOS IPA builds must be executed on a macOS host with Xcode installed." -ForegroundColor DarkYellow
    }
}

# Backend Docker Image Build
if ($Target -eq "all" -or $Target -eq "server" -or $Target -eq "docker") {
    Set-Location $ProjectRoot
    Write-Host "`n[*] Building Backend Production Docker Image (meowgram:1.0.0)..." -ForegroundColor Cyan
    docker build -t meowgram:1.0.0 -t meowgram:latest -f "$ServerDir/Dockerfile" "$ServerDir"
    Write-Host "[OK] Docker image meowgram:1.0.0 built successfully." -ForegroundColor Green
}

Set-Location $ProjectRoot

Write-Host "`n==========================================================" -ForegroundColor Cyan
Write-Host "  meowGram Production Packaging Completed Successfully!" -ForegroundColor Cyan
Write-Host "==========================================================" -ForegroundColor Cyan
