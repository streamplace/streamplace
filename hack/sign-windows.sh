#!/usr/bin/env bash
# Sign the supplied final PE files in place. Local callers supply an Azure access
# token; release CI exchanges its short-lived GitLab identity directly with Entra.
set -euo pipefail
: "${1:?expected at least one PE file}"
: "${AZURE_SIGNING_ACCOUNT:?set the Artifact Signing account name}"
: "${AZURE_SIGNING_PROFILE:?set the Public Trust certificate profile name}"
if [[ ! ${AZURE_SIGNING_ENDPOINT:-} =~ ^https://[a-z0-9]+\.codesigning\.azure\.net/?$ ]]; then
  echo 'AZURE_SIGNING_ENDPOINT must be the regional Azure Artifact Signing HTTPS endpoint' >&2
  exit 1
fi

if [[ -n ${WINDOWS_SIGNING_ID_TOKEN:-} ]]; then
  : "${AZURE_TENANT_ID:?set the Entra tenant ID}" "${AZURE_CLIENT_ID:?set the release application ID}"
  AZURE_SIGNING_ACCESS_TOKEN=$(printf '%s' "$WINDOWS_SIGNING_ID_TOKEN" | curl --fail --silent --show-error \
    --data-urlencode 'client_assertion@-' \
    --data-urlencode "client_id=$AZURE_CLIENT_ID" \
    -d 'grant_type=client_credentials' \
    -d 'scope=https://codesigning.azure.net/.default' \
    -d 'client_assertion_type=urn:ietf:params:oauth:client-assertion-type:jwt-bearer' \
    "https://login.microsoftonline.com/$AZURE_TENANT_ID/oauth2/v2.0/token" | jq -er '.access_token | strings | select(length > 0)')
fi
: "${AZURE_SIGNING_ACCESS_TOKEN:?supply an access token for https://codesigning.azure.net}"
export AZURE_SIGNING_ACCESS_TOKEN
unset WINDOWS_SIGNING_ID_TOKEN

cache=$(cd "$(dirname "$0")/.." && pwd)/.build
jar=$cache/jsign-7.6.jar
sha=6cf58aa080e7ff82656f320cff8d4a3bff30408df8766806dcba3740dd839fa5
if [[ ! -f $jar ]]; then
  mkdir -p "$cache"
  tmp=$(mktemp "$cache/jsign.XXXXXXXX")
  trap 'rm -f "$tmp"' EXIT
  curl --fail --silent --show-error --location --proto '=https' \
    https://github.com/ebourg/jsign/releases/download/7.6/jsign-7.6.jar -o "$tmp"
  echo "$sha  $tmp" | sha256sum --check --status
  mv "$tmp" "$jar"
fi
echo "$sha  $jar" | sha256sum --check --status
exec java -jar "$jar" --storetype TRUSTEDSIGNING --keystore "$AZURE_SIGNING_ENDPOINT" \
  --alias "$AZURE_SIGNING_ACCOUNT/$AZURE_SIGNING_PROFILE" --storepass env:AZURE_SIGNING_ACCESS_TOKEN \
  --replace --alg SHA-256 --tsmode RFC3161 --tsaurl http://timestamp.acs.microsoft.com "$@"
