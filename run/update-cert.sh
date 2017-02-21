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
decrypted="$(echo "$encrypted" | keybase pgp decrypt)"
cert="$(echo "$decrypted" | jq -r '.cert' | base64)"
key="$(echo "$decrypted" | jq -r '.key' | base64)"

kubectl apply -f - << EOF
apiVersion: v1
kind: Secret
type: kubernetes.io/tls
metadata:
  name: $domain
data:
  tls.crt: $cert
  tls.key: $key
EOF

hostsLine="$(minikube ip) $domain"
if ! cat /etc/hosts | grep "$hostsLine" > /dev/null; then
  info "Need to update /etc/hosts"
  info "Please give me sudo powers to run this command:"
  cmd="echo $hostsLine >> /etc/hosts"
  echo "$cmd"
  sudo bash -c "$cmd"
fi
