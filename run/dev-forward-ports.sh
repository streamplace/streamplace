#!/bin/bash

set -o errexit
set -o pipefail
set -o nounset

ingressPod="$(kubectl get pods -o name | grep dev-sp-ingress)"
ingressPod="${ingressPod:5}"
kubectl port-forward "$ingressPod" 10080:80 &
kubectl port-forward "$ingressPod" 10443:443 &
kubectl port-forward "$ingressPod" 11935:1935 &
wait
