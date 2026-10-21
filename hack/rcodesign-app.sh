#!/usr/bin/env bash
#
# Sign a packaged macOS .app with rcodesign, optionally notarize and staple it.
#
#   hack/rcodesign-app.sh path/to/Streamplace.app
#
# Called by the Electron Forge postPackage hook (all-in-one desktop-darwin) and
# by hack/sign-release.sh (split build/sign workflow), so both use the same
# recursive signing and notarization logic.
#
# Environment:
#   DEVELOPER_CERTIFICATE_BASE64, DEVELOPER_CERTIFICATE_PASSWORD
#       Developer ID certificate. Without it rcodesign makes an ad-hoc
#       signature, which macOS will not trust for distribution.
#   APPLE_CODESIGN_API_KEY
#       Path to an App Store Connect API key JSON, for notarization.
#   SIGN_NOTARIZE
#       auto (default: notarize when APPLE_CODESIGN_API_KEY is set), 1, or 0.
#   SIGN_TIMESTAMP_URL
#       rcodesign timestamp server URL. "none" signs without a secure
#       timestamp. Apple requires a timestamp for notarization.
#   SIGN_OFFLINE
#       1 implies SIGN_TIMESTAMP_URL=none and SIGN_NOTARIZE=0. Such a signature
#       cannot be notarized until it is re-signed with a timestamp.
set -euo pipefail

APP="${1:?usage: $0 <App.app>}"
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
ENTITLEMENTS="${ENTITLEMENTS:-$ROOT/js/desktop/entitlements.plist}"

if [ "${SIGN_OFFLINE:-0}" = "1" ]; then
  SIGN_TIMESTAMP_URL="${SIGN_TIMESTAMP_URL:-none}"
  SIGN_NOTARIZE="${SIGN_NOTARIZE:-0}"
fi

TIMESTAMP_ARGS=()
if [ -n "${SIGN_TIMESTAMP_URL:-}" ]; then
  TIMESTAMP_ARGS=(--timestamp-url "$SIGN_TIMESTAMP_URL")
fi

P12_DIR=""
NOTARIZE_DIR=""
cleanup() {
  if [ -n "$P12_DIR" ]; then rm -rf "$P12_DIR"; fi
  if [ -n "$NOTARIZE_DIR" ]; then rm -rf "$NOTARIZE_DIR"; fi
}
trap cleanup EXIT

CERT_ARGS=()
if [ -n "${DEVELOPER_CERTIFICATE_BASE64:-}" ]; then
  P12_DIR="$(mktemp -d "${TMPDIR:-/tmp}/streamplace-p12.XXXXXX")"
  printf '%s' "$DEVELOPER_CERTIFICATE_BASE64" | base64 -d > "$P12_DIR/certificate.p12"
  CERT_ARGS=(--p12-file "$P12_DIR/certificate.p12" --p12-password "${DEVELOPER_CERTIFICATE_PASSWORD:-}")
fi

echo "signing $APP with rcodesign"
rcodesign sign \
  --code-signature-flags runtime \
  --entitlements-xml-path "$ENTITLEMENTS" \
  ${TIMESTAMP_ARGS[@]+"${TIMESTAMP_ARGS[@]}"} \
  ${CERT_ARGS[@]+"${CERT_ARGS[@]}"} \
  "$APP"

NOTARIZE="${SIGN_NOTARIZE:-auto}"
if [ "$NOTARIZE" = "auto" ]; then
  if [ -n "${APPLE_CODESIGN_API_KEY:-}" ]; then
    NOTARIZE=1
  else
    NOTARIZE=0
  fi
fi
if [ "$NOTARIZE" = "1" ]; then
  if [ -z "${APPLE_CODESIGN_API_KEY:-}" ]; then
    echo "APPLE_CODESIGN_API_KEY is required for notarization" >&2
    exit 1
  fi
  if [ "${SIGN_TIMESTAMP_URL:-http://timestamp.apple.com/ts01}" = "none" ]; then
    echo "refusing to notarize an untimestamped signature (SIGN_TIMESTAMP_URL=none)" >&2
    exit 1
  fi
  # Apple only accepts archives; submit a zip, then staple the bundle that the
  # makers distribute.
  NOTARIZE_DIR="$(mktemp -d "${TMPDIR:-/tmp}/streamplace-notarize.XXXXXX")"
  (
    cd "$(dirname "$APP")"
    zip -qry "$NOTARIZE_DIR/Streamplace.zip" "$(basename "$APP")"
  )
  rcodesign notary-submit \
    --api-key-file "$APPLE_CODESIGN_API_KEY" \
    --wait \
    "$NOTARIZE_DIR/Streamplace.zip"
  rcodesign staple "$APP"
  rm -rf "$NOTARIZE_DIR"
  NOTARIZE_DIR=""
fi
