#!/bin/bash

set -o errexit
set -o nounset
set -o pipefail

ROOT="$( cd "$( dirname "${BASH_SOURCE[0]}" )"/.. && pwd )"
source "$ROOT/run/common.sh"

if ! which keybase > /dev/null; then
  echo "Keybase doesn't seem to be installed, can't proceed"
  exit 1
fi

# Get stuff out of values-dev.yaml if it exists
if [[ ! -f "$ROOT/values-dev.yaml" ]]; then
  echo "no values-dev.yaml file found, can't proceed"
  exit 1
fi

domain="$(js-yaml "$ROOT/values-dev.yaml" | jq -r '.global.domain')"
encrypted="$(curl https://sp-dev.club/$domain)"
newHash="$(echo encrypted | openssl sha1)"
if kubectl get secret $domain -o json 2> /dev/null > /dev/null; then
  oldHash="$(kubectl get secret $domain -o json | jq -r '.metadata.annotations.encryptedHash')"
  if [[ "$newHash" == "$oldHash" ]]; then
    echo "Just talked to Streamplace, and they say your TLS cert for $domain is up-to-date. Great!"
    exit 0
  fi
fi
decrypted="$(echo "$encrypted" | keybase pgp decrypt)"
cert="$(echo "$decrypted" | jq -r '.cert' | base64)"
key="$(echo "$decrypted" | jq -r '.key' | base64)"

kubectl apply -f - << EOF
apiVersion: v1
kind: Secret
type: kubernetes.io/tls
metadata:
  name: $domain
  annotations:
    encryptedHash: "$newHash"
data:
  tls.crt: $cert
  tls.key: $key
EOF
