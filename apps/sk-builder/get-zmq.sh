#!/bin/bash

set -o errexit
set -o nounset
set -o pipefail

# Moved this out of the dockerfile so it can be used by the dev scripts too

git clone https://github.com/zeromq/libzmq && \
  cd libzmq && \
  git checkout 464d3fd3f8cf606c99c1a03806fcb3230314b90b && \
  ./autogen.sh && \
  ./configure --prefix="$PREFIX" && \
  make -j 4 && \
  make install &&
  rm -rf libzmq
