#!/bin/bash

set -o errexit
set -o nounset
set -o pipefail

mkfifo /log
cat /log &
/usr/local/nginx/sbin/nginx
