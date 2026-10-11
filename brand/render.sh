#!/bin/sh
# Regenerates the raster logo files from the SVG sources in this folder.
# Needs only Docker: librsvg runs in a disposable Alpine container.
# Afterwards, refresh the platform launcher icons from client/:
#   dart run flutter_launcher_icons
set -eu
root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
docker run --rm -v "$root:/repo" -w /repo alpine:3.21 sh -ec '
  apk add --no-cache rsvg-convert >/dev/null
  r() { rsvg-convert -w "$2" -h "$2" "brand/$1.svg" -o "$3"; }
  r meowgram-icon-full-bleed 1024 client/assets/icon/icon.png
  r meowgram-icon            1024 client/assets/icon/icon_rounded.png
  r meowgram-icon-foreground 1024 client/assets/icon/icon_foreground.png
  r meowgram-icon-monochrome 1024 client/assets/icon/icon_monochrome.png
  r meowgram-icon             512 client/assets/brand/logo.png
  r meowgram-icon             256 brand/meowgram-icon-256.png
  r meowgram-icon             288 client/android/app/src/main/res/drawable-xxhdpi/launch_logo.png
  l=client/ios/Runner/Assets.xcassets/LaunchImage.imageset
  r meowgram-icon             120 $l/LaunchImage.png
  r meowgram-icon             240 $l/LaunchImage@2x.png
  r meowgram-icon             360 $l/LaunchImage@3x.png
'
