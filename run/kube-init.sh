#!/bin/bash

set -o errexit
set -o nounset
set -o pipefail

ROOT="$( cd "$( dirname "${BASH_SOURCE[0]}" )"/.. && pwd )"
source "$ROOT/run/common.sh"

# TODO: have this script work in situations other then https://github.com/streamplace/kube-for-mac

os="$(uname)"

if [[ "$os" != "Darwin" ]]; then
  confirm "This script is untested on non-macs. Continue?"
fi

if ! docker ps > /dev/null; then
  echo "Docker doesn't appear to be running. Make sure 'docker ps' works and try again."
  exit 1
fi

domain="$(js-yaml "$ROOT/values-dev.yaml" | jq -r '.global.domain')"
hostsLine="127.0.0.1 $domain"
if ! cat /etc/hosts | grep "$hostsLine" > /dev/null; then
  info "Need to update /etc/hosts"
  info "Please give me sudo powers to run this command:"
  cmd="echo $hostsLine >> /etc/hosts"
  echo "$cmd"
  sudo bash -c "$cmd"
fi

if ! docker ps | grep kubelet > /dev/null; then
  echo "kubelet doesn't appear to be running."
  echo "We will now use https://github.com/streamplace/kube-for-mac to spin up a local Kubernetes"
  confirm "cluster running on Docker for Mac. That sound good?"
  docker run --rm --privileged -v /:/rootfs streamplace/kube-for-mac
  echo "Adding local kubernetes cluster to $HOME/.kube/config"
  mkdir -p "$HOME/.kube"
  KUBECONFIG="$ROOT/hack/local-kubeconfig:$HOME/.kube/config" kubectl config view > "$HOME/.kube/config"
fi

while ! kubectl get nodes > /dev/null; do
  echo "Waiting for kubectl to be responsive..."
  sleep 2
done
