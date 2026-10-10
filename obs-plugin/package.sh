#!/bin/bash
# Packages a built plugin in the layout OBS loads plugins from:
#   Linux:   extract into ~/.config/obs-studio/plugins/
#   Windows: extract into C:\ProgramData\obs-studio\plugins\
#   macOS:   extract into ~/Library/Application Support/obs-studio/plugins/
# Usage: package.sh <target> <version> <out-dir>

set -euo pipefail

TARGET="$1"
VERSION="$2"
OUT="$(realpath "$3")"
NAME=obs-streamplace
BUILD="$(dirname "$0")/build-$TARGET"
STAGE="$(mktemp -d)"
trap 'rm -rf "$STAGE"' EXIT

case "$TARGET" in
linux-*)
  mkdir -p "$STAGE/$NAME/bin/64bit"
  cp "$BUILD/$NAME.so" "$STAGE/$NAME/bin/64bit/"
  tar -czf "$OUT/$NAME-$VERSION-$TARGET.tar.gz" -C "$STAGE" "$NAME"
  ;;
windows-*)
  mkdir -p "$STAGE/$NAME/bin/64bit"
  cp "$BUILD/$NAME.dll" "$STAGE/$NAME/bin/64bit/"
  rm -f "$OUT/$NAME-$VERSION-$TARGET.zip"
  (cd "$STAGE" && zip -qr "$OUT/$NAME-$VERSION-$TARGET.zip" "$NAME")
  ;;
darwin-*)
  BUNDLE="$STAGE/$NAME.plugin/Contents"
  mkdir -p "$BUNDLE/MacOS"
  cp "$BUILD/$NAME.dylib" "$BUNDLE/MacOS/$NAME"
  cat >"$BUNDLE/Info.plist" <<EOF
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>CFBundleName</key>
	<string>$NAME</string>
	<key>CFBundleIdentifier</key>
	<string>place.stream.$NAME</string>
	<key>CFBundleExecutable</key>
	<string>$NAME</string>
	<key>CFBundlePackageType</key>
	<string>BNDL</string>
	<key>CFBundleInfoDictionaryVersion</key>
	<string>6.0</string>
	<key>CFBundleShortVersionString</key>
	<string>$VERSION</string>
	<key>LSMinimumSystemVersion</key>
	<string>11.0</string>
</dict>
</plist>
EOF
  tar -czf "$OUT/$NAME-$VERSION-$TARGET.tar.gz" -C "$STAGE" "$NAME.plugin"
  ;;
*)
  echo "unknown target $TARGET" >&2
  exit 1
  ;;
esac
