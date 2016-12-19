#!/bin/bash

set -o errexit
set -o nounset
set -o pipefail

docker run \
  --net=host \
  --pid=host \
  --privileged \
  --volume=/:/rootfs:ro \
  --volume=/sys:/sys:ro \
  --volume=/var/run:/var/run:rw \
  --volume=/var/lib/docker/:/var/lib/docker:rw \
  --volume=/var/lib/kubelet/:/var/lib/kubelet:shared \
  --name=kubelet \
  -d \
  gcr.io/google_containers/hyperkube-amd64:v1.5.1 \
  /hyperkube kubelet \
    --address="0.0.0.0" \
    --containerized \
    --hostname-override="127.0.0.1" \
    --api-servers=http://localhost:8080 \
    --pod-manifest-path=/etc/kubernetes/manifests \
    --cluster-dns=10.0.0.10 \
    --cluster-domain=cluster.local \
    --allow-privileged=true --v=2
