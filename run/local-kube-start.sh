#!/bin/bash

set -o errexit
set -o nounset
set -o pipefail

docker run \
  --volume=/:/rootfs:ro \
  --volume=/sys:/sys:ro \
  --volume=/var/lib/docker/:/var/lib/docker:rw \
  --volume=/var/lib/kubelet/:/var/lib/kubelet:rw \
  --volume=/var/run:/var/run:rw \
  --net=host \
  --pid=host \
  --privileged=true \
  --name=kubelet \
  -d \
  gcr.io/google_containers/hyperkube-amd64:v1.3.7 \
  /hyperkube kubelet \
    --containerized \
    --hostname-override="127.0.0.1" \
    --address="0.0.0.0" \
    --api-servers=http://localhost:8080 \
    --config=/etc/kubernetes/manifests \
    --cluster-dns=10.0.0.10 \
    --cluster-domain=cluster.local \
    --allow-privileged=true --v=2
