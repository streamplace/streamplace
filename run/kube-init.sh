#!/bin/bash

set -o errexit
set -o nounset
set -o pipefail

ROOT="$( cd "$( dirname "${BASH_SOURCE[0]}" )"/.. && pwd )"
source "$ROOT/run/common.sh"

# TODO: have this script work in situations other then https://github.com/streamplace/kube-for-mac

os="$(uname)"

if ! docker ps > /dev/null; then
  echo "Docker doesn't appear to be running. Make sure 'docker ps' works and try again."
  exit 1
fi

domain="$(js-yaml "$ROOT/values-dev.yaml" | jq -r '.global.domain')"
ip="$(js-yaml "$ROOT/values-dev.yaml" | jq -r '.global.externalIP')"
hostsLine="$ip $domain"
if ! cat /etc/hosts | grep "$hostsLine" > /dev/null; then
  info "Need to update /etc/hosts"
  info "Please give me sudo powers to run this command:"
  cmd="$ROOT/node_modules/.bin/hostile set $ip $domain"
  echo "$cmd"
  sudo bash -c "$cmd"
fi

if [[ "$os" == "Darwin" ]]; then
  if ! docker ps | grep kubelet > /dev/null; then
    echo "kubelet doesn't appear to be running."
    echo "We will now use https://github.com/streamplace/kube-for-mac to spin up a local Kubernetes"
    confirm "cluster running on Docker for Mac. That sound good?"
    docker rm -f "/docker-kube-for-mac-start" || echo -n ""
    curl https://raw.githubusercontent.com/streamplace/kube-for-mac/master/run-docker-kube-for-mac.sh | bash -s start
    echo "Adding local kubernetes cluster to $HOME/.kube/config"
    mkdir -p "$HOME/.kube"
  fi
fi

while ! kubectl get nodes > /dev/null; do
  echo "Waiting for kubectl to be responsive..."
  sleep 2
done

kubectl apply -f - << EOF
apiVersion: v1
kind: ConfigMap
metadata:
  name: kube-dns
  namespace: kube-system
data:
  upstreamNameservers: |
    ["8.8.8.8", "8.8.4.4"]
EOF

# while ! kubectl get deployment -n kube-system kubernetes-dashboard > /dev/null; do
#   echo "Waiting for kubernetes-dashboard pod..."
#   sleep 2
# done

# patch="$(
#   jq -n '.spec.template.spec.containers[0].ports[0] = {"hostPort": 9090, "containerPort": 9090}' |
#   jq '.spec.template.spec.containers[0].name = "kubernetes-dashboard"'
# )"
# kubectl -n kube-system patch deployment kubernetes-dashboard -p "$patch"
