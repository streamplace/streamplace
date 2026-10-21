#!/bin/bash
# Sign a cross-compiled macOS binary with rcodesign, and optionally notarize it.
#
# Environment:
#   DEVELOPER_CERTIFICATE_BASE64, DEVELOPER_CERTIFICATE_PASSWORD
#   APPLE_CODESIGN_API_KEY   path to an App Store Connect API key JSON
#   SIGN_NOTARIZE            auto (default: notarize when the key is set), 1, 0
#   SIGN_TIMESTAMP_URL       rcodesign TSA URL; "none" signs offline
#   SIGN_OFFLINE             1 implies SIGN_TIMESTAMP_URL=none and SIGN_NOTARIZE=0
#   SIGN_NOTARY_WAIT         1 waits for the notarization result
#
# With no certificate this is a no-op, matching the build-only workflow.
set -euo pipefail

CERT_DIR=""
NOTARY_DIR=""
cleanup() {
  if [ -n "$CERT_DIR" ]; then rm -rf "$CERT_DIR"; fi
  if [ -n "$NOTARY_DIR" ]; then rm -rf "$NOTARY_DIR"; fi
}
trap cleanup EXIT

if [ "${SIGN_OFFLINE:-0}" = "1" ]; then
  SIGN_TIMESTAMP_URL="${SIGN_TIMESTAMP_URL:-none}"
  SIGN_NOTARIZE="${SIGN_NOTARIZE:-0}"
fi

if [[ "${DEVELOPER_CERTIFICATE_BASE64:-}" == "" ]]; then
  echo "DEVELOPER_CERTIFICATE_BASE64 is not set, skipping codesign"
  exit 0
fi

CERT_DIR="$(mktemp -d "${TMPDIR:-/tmp}/streamplace-cert.XXXXXX")"
printf '%s' "$DEVELOPER_CERTIFICATE_BASE64" | base64 -d >"$CERT_DIR/certificate.p12"

TIMESTAMP_ARGS=()
if [[ -n "${SIGN_TIMESTAMP_URL:-}" ]]; then
  TIMESTAMP_ARGS=(--timestamp-url "$SIGN_TIMESTAMP_URL")
fi

rcodesign sign \
  --p12-file "$CERT_DIR/certificate.p12" \
  --p12-password "${DEVELOPER_CERTIFICATE_PASSWORD:-}" \
  --code-signature-flags runtime \
  ${TIMESTAMP_ARGS[@]+"${TIMESTAMP_ARGS[@]}"} \
  "$1"
rm -rf "$CERT_DIR"
CERT_DIR=""

NOTARIZE="${SIGN_NOTARIZE:-auto}"
if [[ -z "${APPLE_CODESIGN_API_KEY:-}" ]]; then
  NOTARIZE=0
fi
if [[ "$NOTARIZE" == "1" ]]; then
  if [[ "${SIGN_TIMESTAMP_URL:-http://timestamp.apple.com/ts01}" == "none" ]]; then
    echo "refusing to notarize an untimestamped signature (SIGN_TIMESTAMP_URL=none)" >&2
    exit 1
  fi
  NOTARY_DIR="$(mktemp -d "${TMPDIR:-/tmp}/streamplace-notary.XXXXXX")"
  (
    cd "$(dirname "$1")"
    zip -q "$NOTARY_DIR/asset.zip" "$(basename "$1")"
  )
  WAIT_ARGS=()
  if [[ "${SIGN_NOTARY_WAIT:-0}" == "1" ]]; then
    WAIT_ARGS=(--wait)
  fi
  rcodesign notary-submit \
    --api-key-file "$APPLE_CODESIGN_API_KEY" \
    ${WAIT_ARGS[@]+"${WAIT_ARGS[@]}"} \
    "$NOTARY_DIR/asset.zip"
  rm -rf "$NOTARY_DIR"
  NOTARY_DIR=""
fi
