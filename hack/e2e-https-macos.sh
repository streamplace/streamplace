#!/usr/bin/env bash
# Install/remove a persistent, loopback-only 443 -> 38443 relay for local OAuth.
# launchd owns the privileged listener; each socat connection runs as nobody.
set -euo pipefail

LABEL=network.streamplace.e2e-https
PLIST="/Library/LaunchDaemons/$LABEL.plist"
[ "$(uname)" = Darwin ] || { echo "this helper requires macOS" >&2; exit 1; }
[ "$(id -u)" = 0 ] || { echo "run with sudo: sudo bash $0 ${1:-install}" >&2; exit 1; }

case "${1:-install}" in
  install)
    SOCAT="${SOCAT_BIN:-$(command -v socat || true)}"
    if [ -z "$SOCAT" ]; then
      for candidate in /opt/homebrew/bin/socat /usr/local/bin/socat; do
        if [ -x "$candidate" ]; then SOCAT="$candidate"; break; fi
      done
    fi
    [ -x "$SOCAT" ] || { echo "socat missing — install with 'brew install socat'" >&2; exit 1; }
    TEMP="$(mktemp)"
    trap 'rm -f "$TEMP"' EXIT
    cat > "$TEMP" <<'PLIST'
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0"><dict>
  <key>Label</key><string>network.streamplace.e2e-https</string>
  <key>ProgramArguments</key><array>
    <string>/usr/bin/false</string>
    <string>STDIO</string>
    <string>TCP4:127.0.0.1:38443</string>
  </array>
  <key>UserName</key><string>nobody</string>
  <key>GroupName</key><string>nobody</string>
  <key>inetdCompatibility</key><dict><key>Wait</key><false/></dict>
  <key>ThrottleInterval</key><integer>0</integer>
  <key>StandardErrorPath</key><string>/var/log/streamplace-e2e-https.log</string>
  <key>Sockets</key><dict><key>Listener</key><dict>
    <key>SockFamily</key><string>IPv4</string>
    <key>SockType</key><string>stream</string>
    <key>SockNodeName</key><string>127.0.0.1</string>
    <key>SockServiceName</key><string>443</string>
  </dict></dict>
</dict></plist>
PLIST
    /usr/libexec/PlistBuddy -c "Set :ProgramArguments:0 $SOCAT" "$TEMP"
    plutil -lint "$TEMP"
    if launchctl print "system/$LABEL" >/dev/null 2>&1; then
      launchctl bootout "system/$LABEL"
    fi
    install -o root -g wheel -m 644 "$TEMP" "$PLIST"
    touch /var/log/streamplace-e2e-https.log
    chown nobody:nobody /var/log/streamplace-e2e-https.log
    chmod 644 /var/log/streamplace-e2e-https.log
    launchctl bootstrap system "$PLIST"
    echo "Installed $LABEL: 127.0.0.1:443 -> 127.0.0.1:38443 (persists across reboots)"
    ;;
  uninstall)
    if launchctl print "system/$LABEL" >/dev/null 2>&1; then
      launchctl bootout "system/$LABEL"
    fi
    rm -f "$PLIST"
    echo "Removed $LABEL"
    ;;
  *) echo "usage: sudo bash $0 [install|uninstall]" >&2; exit 1 ;;
esac
