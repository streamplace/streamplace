FROM ubuntu:22.04 AS builder

ARG TARGETARCH
ENV TARGETARCH $TARGETARCH

ARG DOCKERFILE_HASH
ENV DOCKERFILE_HASH $DOCKERFILE_HASH

ENV GO_VERSION 1.24.2
ENV NODE_VERSION 22.15.0
ENV DEBIAN_FRONTEND noninteractive

RUN apt update && apt install -y ca-certificates

ADD docker/sources.list /etc/apt/sources.list
ADD docker/winehq.key /etc/apt/keyrings/winehq-archive.key
RUN dpkg --add-architecture i386 && dpkg --add-architecture arm64

# Haven't automated it yet, so here's my instructors for mirroring winehq:
# /etc/apt/mirror.list:
# deb-i386 https://dl.winehq.org/wine-builds/ubuntu jammy main
# deb-all https://dl.winehq.org/wine-builds/ubuntu jammy main
# deb-amd64 [arch=amd64,i386 signed-by=/etc/apt/keyrings/winehq-archive.key] https://dl.winehq.org/wine-builds/ubuntu jammy main
#
# go install github.com/minio/mc@latest
# mc alias set streamplace-crap https://storage.googleapis.com/ ACCESS_KEY SECRET_KEY
# apt-mirror
# mc mirror --overwrite /var/spool/apt-mirror/mirror/dl.winehq.org/ streamplace-crap/streamplace-crap/dl.winehq.org/

RUN apt update \
  && apt install -y build-essential curl git openjdk-17-jdk unzip jq g++ python3-pip ninja-build \
  gcc-aarch64-linux-gnu g++-aarch64-linux-gnu clang lld qemu-user-static pkg-config \
  nasm gcc-mingw-w64-x86-64 g++-mingw-w64-x86-64 mingw-w64-tools zip bison flex expect \
  mono-runtime nuget mono-xsp4 squashfs-tools \
  libc6:arm64 libstdc++6:arm64 \
  && apt install -y --install-recommends winehq-stable \
  && pip install meson tomli \
  && curl -L --fail https://go.dev/dl/go$GO_VERSION.linux-$TARGETARCH.tar.gz -o go.tar.gz \
  && tar -C /usr/local -xf go.tar.gz \
  && rm go.tar.gz
ENV PATH $PATH:/usr/local/go/bin:/root/go/bin:/root/.cargo/bin

RUN export NODEARCH="$TARGETARCH" \
  && if [ "$TARGETARCH" = "amd64" ]; then export NODEARCH="x64"; fi \
  && curl -L --fail https://nodejs.org/dist/v$NODE_VERSION/node-v$NODE_VERSION-linux-$NODEARCH.tar.xz -o node.tar.gz \
  && tar -xf node.tar.gz \
  && cp -r node-v$NODE_VERSION-linux-$NODEARCH/* /usr/local \
  && rm -rf node.tar.gz node-v$NODE_VERSION-linux-$NODEARCH

RUN npm install -g yarn

ARG ANDROID_SDK_VERSION=11076708
ENV ANDROID_HOME /opt/android-sdk
RUN mkdir -p ${ANDROID_HOME}/cmdline-tools && \
  curl -L -O https://dl.google.com/android/repository/commandlinetools-linux-${ANDROID_SDK_VERSION}_latest.zip && \
  unzip *tools*linux*.zip -d ${ANDROID_HOME}/cmdline-tools && \
  mv ${ANDROID_HOME}/cmdline-tools/cmdline-tools ${ANDROID_HOME}/cmdline-tools/tools && \
  rm *tools*linux*.zip && \
  curl -L https://raw.githubusercontent.com/thyrlian/AndroidSDK/bfcbf0cdfd6bb1ef45579e6ddc4d3876264cbdd1/android-sdk/license_accepter.sh | bash

RUN curl --proto '=https' --tlsv1.2 -sSf https://sh.rustup.rs > rustup.sh \
  && bash rustup.sh -y \
  && rustup target add aarch64-unknown-linux-gnu \
  && rustup target add x86_64-unknown-linux-gnu \
  && rustup target add x86_64-pc-windows-gnu \
  && rustup target add x86_64-apple-darwin \
  && rustup target add aarch64-apple-darwin \
  && rm rustup.sh

RUN go env -w GOTOOLCHAIN=go$GO_VERSION

# FROM builder AS cached-builder
# ARG CI_COMMIT_BRANCH=next
# ENV CI_COMMIT_BRANCH $CI_COMMIT_BRANCH
# WORKDIR /cached-build
# RUN git clone https://git.stream.place/streamplace/streamplace \
#   && cd streamplace \
#   && make version install check app android -j$(nproc) \
#   && make node \
#   && cd .. \
#   && rm -rf streamplace

RUN curl -L https://github.com/golangci/golangci-lint/releases/download/v2.1.6/golangci-lint-2.1.6-linux-amd64.tar.gz -o golangci-lint.tar.gz \
  && tar -xf golangci-lint.tar.gz \
  && mv golangci-lint-2.1.6-linux-amd64/golangci-lint /usr/local/bin/ \
  && rm -rf golangci-lint.tar.gz golangci-lint-2.1.6-linux-amd64

LABEL org.opencontainers.image.authors="support@stream.place"
