# syntax=docker/dockerfile:1
ARG TARGETARCH

FROM --platform=linux/$TARGETARCH ubuntu:22.04 AS builder-android

ARG TARGETARCH
ENV TARGETARCH $TARGETARCH

ARG DOCKERFILE_HASH
ENV DOCKERFILE_HASH $DOCKERFILE_HASH

ENV GO_VERSION 1.24.2
ENV NODE_VERSION 22.15.0
ENV DEBIAN_FRONTEND noninteractive

RUN apt update && apt install -y curl

RUN curl --fail https://raw.githubusercontent.com/streamplace/streamplace/ebaecf20577132b2206d2589c378b07bbbfd96ab/docker/sources.list -o /etc/apt/sources.list
RUN curl --fail https://raw.githubusercontent.com/streamplace/streamplace/ebaecf20577132b2206d2589c378b07bbbfd96ab/docker/winehq.key -o /etc/apt/keyrings/winehq-archive.key
RUN curl --fail https://raw.githubusercontent.com/streamplace/streamplace/ebaecf20577132b2206d2589c378b07bbbfd96ab/docker/llvm-snapshot.key -o /etc/apt/keyrings/llvm-snapshot.key

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
  && apt install -y curl git openjdk-17-jdk unzip jq python3-pip ninja-build \
  && pip install meson tomli

RUN curl -L --fail https://go.dev/dl/go$GO_VERSION.linux-$TARGETARCH.tar.gz -o go.tar.gz \
  && tar -C /usr/local -xf go.tar.gz \
  && rm go.tar.gz

RUN export NODEARCH="$TARGETARCH" \
  && if [ "$TARGETARCH" = "amd64" ]; then export NODEARCH="x64"; fi \
  && curl -L --fail https://nodejs.org/dist/v$NODE_VERSION/node-v$NODE_VERSION-linux-$NODEARCH.tar.xz -o node.tar.gz \
  && tar -xf node.tar.gz \
  && cp -r node-v$NODE_VERSION-linux-$NODEARCH/* /usr/local \
  && rm -rf node.tar.gz node-v$NODE_VERSION-linux-$NODEARCH

RUN npm install -g corepack@latest

ARG ANDROID_SDK_VERSION=11076708
ENV ANDROID_HOME /opt/android-sdk
RUN mkdir -p ${ANDROID_HOME}/cmdline-tools && \
  curl -L -O https://dl.google.com/android/repository/commandlinetools-linux-${ANDROID_SDK_VERSION}_latest.zip && \
  unzip *tools*linux*.zip -d ${ANDROID_HOME}/cmdline-tools && \
  mv ${ANDROID_HOME}/cmdline-tools/cmdline-tools ${ANDROID_HOME}/cmdline-tools/tools && \
  rm *tools*linux*.zip && \
  curl -L https://raw.githubusercontent.com/thyrlian/AndroidSDK/bfcbf0cdfd6bb1ef45579e6ddc4d3876264cbdd1/android-sdk/license_accepter.sh | bash

RUN export NODEARCH="$TARGETARCH" \
  && if [ "$TARGETARCH" = "amd64" ]; then export NODEARCH="x64"; fi \
  && curl -L --fail https://nodejs.org/dist/v$NODE_VERSION/node-v$NODE_VERSION-linux-$NODEARCH.tar.xz -o node.tar.gz \
  && tar -xf node.tar.gz \
  && cp -r node-v$NODE_VERSION-linux-$NODEARCH/* /usr/local \
  && rm -rf node.tar.gz node-v$NODE_VERSION-linux-$NODEARCH

RUN if [ "$TARGETARCH" = "amd64" ]; then export ANDROIDARCH="x86_64"; fi \
  && if [ "$TARGETARCH" = "arm64" ]; then export ANDROIDARCH="arm64-v8a"; fi \
  && $ANDROID_HOME/cmdline-tools/tools/bin/sdkmanager --install emulator \
  && $ANDROID_HOME/cmdline-tools/tools/bin/sdkmanager "system-images;android-28;default;$ANDROIDARCH" \
  && $ANDROID_HOME/cmdline-tools/tools/bin/avdmanager create avd -n Pixel_API_28_AOSP -d pixel --package "system-images;android-28;default;$ANDROIDARCH"

FROM builder-android AS builder-no-darwin

RUN dpkg --add-architecture i386 && dpkg --add-architecture arm64 && dpkg --add-architecture amd64

ENV PATH /usr/lib/llvm-21/bin:$PATH:/usr/local/go/bin:/root/go/bin:/root/.cargo/bin

RUN apt update && apt install -y \
  build-essential g++ apksigner \
  gcc-aarch64-linux-gnu g++-aarch64-linux-gnu qemu-user-static pkg-config \
  nasm gcc-mingw-w64-x86-64 g++-mingw-w64-x86-64 mingw-w64-tools zip bison flex expect \
  mono-runtime nuget mono-xsp4 squashfs-tools \
  libc6:arm64 libstdc++6:arm64 \
  cmake libssl-dev libssl-dev:arm64 \
  ruby-rubygems

RUN echo 'deb [arch=amd64,i386 signed-by=/etc/apt/keyrings/winehq-archive.key] https://storage.googleapis.com/streamplace-crap/dl.winehq.org/wine-builds/ubuntu/ jammy main' >> /etc/apt/sources.list \
  && echo 'deb [arch=amd64 signed-by=/etc/apt/keyrings/llvm-snapshot.key] http://apt.llvm.org/jammy/ llvm-toolchain-jammy-21 main' >> /etc/apt/sources.list \
  && apt update \
  && apt install -y --install-recommends winehq-stable \
  clang-21 lldb-21 lld-21 clangd-21

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

RUN gem install fpm
ENV APTLY_VERSION 1.6.2
RUN curl --fail -L https://github.com/aptly-dev/aptly/releases/download/v${APTLY_VERSION}/aptly_${APTLY_VERSION}_linux_amd64.zip -o aptly.zip \
  && unzip aptly.zip \
  && mv aptly_${APTLY_VERSION}_linux_amd64/aptly /usr/local/bin/ \
  && rm -rf aptly.zip aptly_${APTLY_VERSION}_linux_amd64

ENV COREPACK_ENABLE_DOWNLOAD_PROMPT=false

FROM builder-no-darwin AS builder

WORKDIR /osxcross

RUN git clone https://github.com/tpoechtrager/osxcross.git .
# RUN UNATTENDED=1 ./build_apple_clang.sh
ENV MAC_SDK_VERSION 15.4
RUN curl -L --fail https://github.com/joseluisq/macosx-sdks/releases/download/$MAC_SDK_VERSION/MacOSX$MAC_SDK_VERSION.sdk.tar.xz -o /osxcross/tarballs/MacOSX$MAC_SDK_VERSION.sdk.tar.xz
RUN UNATTENDED=1 ./build.sh
RUN cargo install apple-codesign
ENV PATH /osxcross/target/bin:$PATH

LABEL org.opencontainers.image.authors="support@stream.place"
