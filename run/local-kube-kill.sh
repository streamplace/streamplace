#!/bin/bash

# kill our kubelet
docker rm -f kubelet

# kill all K8S containers running
docker ps | grep k8s | cut -d " " -f 1 | xargs --no-run-if-empty -L 1 docker rm -f
