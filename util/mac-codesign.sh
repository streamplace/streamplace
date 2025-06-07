#!/bin/bash

set -euo pipefail

CODESIGN="$(command -v codesign || echo -n "/usr/bin/codesign")"
NOTARIZATION_FILE="/tmp/LP_NOTARIZATION_${RANDOM}.zip"
CERTIFICATE_FILE="certificate.csr"
KEYCHAIN_NAME="streamplace.keychain"
KEYCHAIN_FILE=""

function livepeer-keychain() {
  # Create and unlock a custom temporary keychain for codesigning and
  # notarization purpose
  local password="$(uuidgen)"
  if [[ "${KEYCHAIN_PASSWORD}" == "" ]]; then
    KEYCHAIN_PASSWORD="$password"
  fi
  security create-keychain -p "$KEYCHAIN_PASSWORD" "$KEYCHAIN_NAME" || echo 'already exists'
  security default-keychain -s "$KEYCHAIN_NAME"
  security unlock-keychain -p "$KEYCHAIN_PASSWORD" "$KEYCHAIN_NAME"
  if [[ "${KEYCHAIN_FILE:-}" == "" ]]; then
    KEYCHAIN_FILE="$(security default-keychain | sed -e 's:^["\t ]*::;s:["\t ]*$::')"
  fi
  echo "${DEVELOPER_CERTIFICATE_BASE64}" | base64 -d >"$CERTIFICATE_FILE"
  security unlock-keychain -p "$KEYCHAIN_PASSWORD" "$KEYCHAIN_NAME"
  security import "${CERTIFICATE_FILE}" -f pkcs12 -k "$KEYCHAIN_NAME" -T "$CODESIGN" -P "${DEVELOPER_CERTIFICATE_PASSWORD}"
  security set-key-partition-list -S "apple-tool:,apple:,codesign:" -s -k "$KEYCHAIN_PASSWORD" "$KEYCHAIN_NAME"
  rm -f "${CERTIFICATE_FILE}"
}

function livepeer-codesign() {
  $CODESIGN --force --sign "${DEVELOPER_CERTIFICATE_ID}" -o runtime "${BINARY_PATH}"
  zip -9r "${NOTARIZATION_FILE}" "${BINARY_PATH}"
}

function livepeer-notarize() {
  local keychain_profile="lp-notarize_${RANDOM}"
  security default-keychain -s "$KEYCHAIN_NAME"
  security unlock-keychain -p "$KEYCHAIN_PASSWORD" "$KEYCHAIN_NAME"
  if [[ "$KEYCHAIN_FILE" == "" ]]; then
    KEYCHAIN_FILE="$(security default-keychain | sed -e 's:^["\t ]*::;s:["\t ]*$::')"
  fi
  xcrun notarytool store-credentials \
    --verbose \
    --validate \
    --apple-id "$NOTARIZATION_EMAIL" \
    --password "$NOTARIZATION_PASSWORD" \
    --team-id "$NOTARIZATION_TEAM_ID" \
    --keychain "$KEYCHAIN_FILE" \
    "$keychain_profile"

  xcrun notarytool submit \
    --keychain-profile "$keychain_profile" \
    --keychain "$KEYCHAIN_FILE" \
    --verbose \
    "${NOTARIZATION_FILE}"
  rm -f "${NOTARIZATION_FILE}"
}

export BINARY_PATH="${1:-}"
livepeer-keychain
if [[ "${BINARY_PATH:-}" != "" ]]; then
  livepeer-codesign
  livepeer-notarize
  codesign -dvv "$BINARY_PATH"
fi
