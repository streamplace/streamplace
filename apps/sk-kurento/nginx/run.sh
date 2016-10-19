#!/bin/bash

set -o errexit
set -o nounset
set -o pipefail

docker run -it --rm --net host \
  -v /home/root/letsencrypt/archive/drumstick.iame.li/fullchain1.pem:/ssl/certchain.pem \
  -v /home/root/letsencrypt/keys/0000_key-certbot.pem:/ssl/key.pem \
  -v /home/root/code/streamkitchen/apps/sk-kurento/nginx/nginx.conf:/etc/nginx/nginx.conf \
  nginx
