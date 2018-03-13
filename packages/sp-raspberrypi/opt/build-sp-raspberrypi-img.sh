#!/bin/bash

set -o errexit
set -o nounset
set -o pipefail
set -x
DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
IMAGE="/image/raspbian.img"
(
  offset="$(fdisk -lu --bytes $IMAGE | grep raspbian.img2 | awk '{ print $2 }')"
  offset=$(($offset * 512))
  mkdir -p /mount
  mount -o loop,offset=$offset $IMAGE /mount
  mkdir -p /mount/opt/streamplace
  rsync -arv . /mount/opt/streamplace

  # create service
  ln -s \
    /opt/streamplace/streamplace.service \
    /mount/etc/systemd/system/streamplace.service

  # simulates `systemctl enable streamplace`
  ln -s \
    /etc/systemd/system/streamplace.service \
    /mount/etc/systemd/system/multi-user.target.wants/streamplace.service
)
