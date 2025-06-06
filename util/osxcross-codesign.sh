#!/bin/bash

set -euo pipefail

CERTIFICATE_FILE="certificate.p12"
if [[ "${DEVELOPER_CERTIFICATE_BASE64:-}" == "" ]]; then
  echo "DEVELOPER_CERTIFICATE_BASE64 is not set, skipping codesign"
  exit 0
fi
echo "${DEVELOPER_CERTIFICATE_BASE64}" | base64 -d >"$CERTIFICATE_FILE"
rcodesign sign \
  --p12-file "$CERTIFICATE_FILE" \
  --p12-password "${DEVELOPER_CERTIFICATE_PASSWORD}" \
  --code-signature-flags runtime \
  "$1"
rm -f "$CERTIFICATE_FILE"

if [[ "$APPLE_CODESIGN_API_KEY:-" != "" ]]; then
  zip "$1.zip" "$1"
  rcodesign notary-submit --api-key-file "$APPLE_CODESIGN_API_KEY" "$1.zip"
fi
