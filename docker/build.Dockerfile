# syntax=docker/dockerfile:1
FROM ubuntu:22.04 AS builder-no-darwin

ARG TARGETARCH
ENV TARGETARCH $TARGETARCH

ARG DOCKERFILE_HASH
ENV DOCKERFILE_HASH $DOCKERFILE_HASH

# You'll probably need to bump golangci-lint if you bump go
ENV GO_VERSION 1.27.3
ENV GOLANGCI_LINT_VERSION 2.15.0
ENV NODE_VERSION 26.13.0
ENV DEBIAN_FRONTEND noninteractive

ADD docker/sources.list /etc/apt/sources.list
ADD docker/winehq.key /etc/apt/keyrings/winehq-archive.key
ADD docker/llvm-snapshot.key /etc/apt/keyrings/llvm-snapshot.key

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
  && apt install -y build-essential curl git openjdk-21-jdk unzip jq g++ python3-pip ninja-build \
  gcc-aarch64-linux-gnu g++-aarch64-linux-gnu qemu-user-static pkg-config \
  nasm gcc-mingw-w64-x86-64 g++-mingw-w64-x86-64 mingw-w64-tools zip bison flex expect \
  mono-runtime nuget mono-xsp4 squashfs-tools \
  libc6:arm64 libstdc++6:arm64 \
  cmake libssl-dev libssl-dev:arm64 libbz2-dev zlib1g-dev hfsprogs hfsplus \
  ruby-rubygems postgresql sudo \
  && pip install meson==1.11.2 tomli \
  && curl -L --fail https://go.dev/dl/go$GO_VERSION.linux-$TARGETARCH.tar.gz -o go.tar.gz \
  && tar -C /usr/local -xf go.tar.gz \
  && rm go.tar.gz

# freedesktop.org's HTTP/2 Git responses can terminate before the pack is complete.
RUN git config --global http.https://gitlab.freedesktop.org/.version HTTP/1.1

# GLib's WinRT package parser needs current MinGW headers and import libraries.
# Keep the existing GNU C++/MSVCRT ABI used by Go and Rust native dependencies.
ENV MINGW_W64_VERSION 14.0.0
RUN curl --fail -L https://github.com/mingw-w64/mingw-w64/archive/refs/tags/v${MINGW_W64_VERSION}.tar.gz -o /tmp/mingw-w64.tar.gz \
  && echo "d71cc644cd5a37c337f2719f3e0c79d89e8d8d5fb9e2952a62d3fa23623dc137  /tmp/mingw-w64.tar.gz" | sha256sum -c - \
  && tar -xf /tmp/mingw-w64.tar.gz -C /tmp \
  && mkdir /tmp/mingw-headers /tmp/mingw-crt /tmp/mingw-winpthreads \
  && cd /tmp/mingw-headers \
  && ../mingw-w64-${MINGW_W64_VERSION}/mingw-w64-headers/configure --host=x86_64-w64-mingw32 --prefix=/usr/x86_64-w64-mingw32 --with-default-msvcrt=msvcrt \
  && make install \
  && cd /tmp/mingw-crt \
  && ../mingw-w64-${MINGW_W64_VERSION}/mingw-w64-crt/configure --host=x86_64-w64-mingw32 --prefix=/usr/x86_64-w64-mingw32 --disable-lib32 --enable-lib64 --with-default-msvcrt=msvcrt \
  && make -j"$(nproc)" \
  && make install \
  && cd /tmp/mingw-winpthreads \
  && ../mingw-w64-${MINGW_W64_VERSION}/mingw-w64-libraries/winpthreads/configure --host=x86_64-w64-mingw32 --prefix=/usr/x86_64-w64-mingw32 \
  && make -j"$(nproc)" \
  && make install \
  && cd / \
  && rm -rf /tmp/mingw-w64.tar.gz /tmp/mingw-w64-${MINGW_W64_VERSION} /tmp/mingw-headers /tmp/mingw-crt /tmp/mingw-winpthreads

# Rust's current COFF objects use exclusion directives unknown to binutils 2.38.
ENV MINGW_BINUTILS_VERSION 2.47
RUN curl --fail -L https://ftp.gnu.org/gnu/binutils/binutils-${MINGW_BINUTILS_VERSION}.tar.xz -o /tmp/mingw-binutils.tar.xz \
  && echo "154ab23b60070e8f27013c22977f1129425d67d1e8acd6e13010e617811e4cff  /tmp/mingw-binutils.tar.xz" | sha256sum -c - \
  && tar -xf /tmp/mingw-binutils.tar.xz -C /tmp \
  && mkdir /tmp/mingw-binutils-build \
  && cd /tmp/mingw-binutils-build \
  && ../binutils-${MINGW_BINUTILS_VERSION}/configure --target=x86_64-w64-mingw32 --prefix=/usr --disable-nls --disable-gprofng \
  && make -j"$(nproc)" \
  && make install \
  && cd / \
  && rm -rf /tmp/mingw-binutils.tar.xz /tmp/binutils-${MINGW_BINUTILS_VERSION} /tmp/mingw-binutils-build

# Builds DMGs from macOS app bundles on Linux (hack/make-dmg.sh), so the
# desktop-darwin job no longer needs a Mac. hfsprogs/hfsplus create the HFS+
# image; libdmg-hfsplus wraps it in a UDIF.
RUN git clone https://github.com/mozilla/libdmg-hfsplus /tmp/libdmg-hfsplus \
  && cd /tmp/libdmg-hfsplus \
  && git checkout e18579b391950589b9ed16ee630c54ce4319bb29 \
  && cmake . \
  && make -j"$(nproc)" \
  && make install \
  && rm -rf /tmp/libdmg-hfsplus

RUN echo 'deb [arch=amd64,i386 signed-by=/etc/apt/keyrings/winehq-archive.key] https://storage.googleapis.com/streamplace-crap/dl.winehq.org/wine-builds/ubuntu/ jammy main' >> /etc/apt/sources.list \
  && echo 'deb [arch=amd64 signed-by=/etc/apt/keyrings/llvm-snapshot.key] https://apt.llvm.org/jammy/ llvm-toolchain-jammy-23 main' >> /etc/apt/sources.list \
  && apt update \
  && apt install -y --install-recommends winehq-stable \
  clang-23 lldb-23 lld-23 clangd-23

ENV PATH /usr/lib/llvm-23/bin:$PATH:/usr/local/go/bin:/root/go/bin:/root/.cargo/bin

RUN export NODEARCH="$TARGETARCH" \
  && if [ "$TARGETARCH" = "amd64" ]; then export NODEARCH="x64"; fi \
  && curl -L --fail https://nodejs.org/dist/v$NODE_VERSION/node-v$NODE_VERSION-linux-$NODEARCH.tar.xz -o node.tar.gz \
  && tar -xf node.tar.gz \
  && cp -r node-v$NODE_VERSION-linux-$NODEARCH/* /usr/local \
  && rm -rf node.tar.gz node-v$NODE_VERSION-linux-$NODEARCH

RUN npm install -g corepack@latest

ARG ANDROID_SDK_VERSION=16111833
ENV ANDROID_HOME /opt/android-sdk
RUN mkdir -p ${ANDROID_HOME}/cmdline-tools && \
  curl -L -O https://dl.google.com/android/repository/commandlinetools-linux-${ANDROID_SDK_VERSION}_latest.zip && \
  unzip *tools*linux*.zip -d ${ANDROID_HOME}/cmdline-tools && \
  mv ${ANDROID_HOME}/cmdline-tools/cmdline-tools ${ANDROID_HOME}/cmdline-tools/tools && \
  rm *tools*linux*.zip && \
  curl -L https://raw.githubusercontent.com/thyrlian/AndroidSDK/bfcbf0cdfd6bb1ef45579e6ddc4d3876264cbdd1/android-sdk/license_accepter.sh | bash

RUN curl --proto '=https' --tlsv1.2 -sSf https://sh.rustup.rs > rustup.sh \
  && bash rustup.sh -y --default-toolchain 1.99.1 \
  && rustup target add aarch64-unknown-linux-gnu \
  && rustup target add x86_64-unknown-linux-gnu \
  && rustup target add x86_64-pc-windows-gnu \
  && rustup target add x86_64-apple-darwin \
  && rustup target add aarch64-apple-darwin \
  && rustup target add wasm32-wasip1 \
  && rustup target add wasm32-unknown-unknown \
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

RUN curl -L https://github.com/golangci/golangci-lint/releases/download/v${GOLANGCI_LINT_VERSION}/golangci-lint-${GOLANGCI_LINT_VERSION}-linux-amd64.tar.gz -o golangci-lint.tar.gz \
  && tar -xf golangci-lint.tar.gz \
  && mv golangci-lint-${GOLANGCI_LINT_VERSION}-linux-amd64/golangci-lint /usr/local/bin/ \
  && rm -rf golangci-lint.tar.gz golangci-lint-${GOLANGCI_LINT_VERSION}-linux-amd64

RUN gem install fpm
ENV APTLY_VERSION 1.6.3
RUN curl --fail -L https://github.com/aptly-dev/aptly/releases/download/v${APTLY_VERSION}/aptly_${APTLY_VERSION}_linux_amd64.zip -o aptly.zip \
  && unzip aptly.zip \
  && mv aptly_${APTLY_VERSION}_linux_amd64/aptly /usr/local/bin/ \
  && rm -rf aptly.zip aptly_${APTLY_VERSION}_linux_amd64

ENV COREPACK_ENABLE_DOWNLOAD_PROMPT=false
ENV STREAMPLACE_TEST_POSTGRES_COMMAND="install -d -o postgres -g postgres -m 3775 /var/run/postgresql/14-main.pg_stat_tmp && sudo -u postgres /usr/lib/postgresql/14/bin/postgres -D /etc/postgresql/14/main/"
ENV STREAMPLACE_TEST_POSTGRES_URL="postgresql://postgres:postgres@localhost:5432/streamplace"
# allow all postgres connections
RUN bash -c 'echo -en "local   all             postgres                                peer\nhost    all             all             0.0.0.0/0            trust\n" > /etc/postgresql/14/main/pg_hba.conf'

FROM builder-no-darwin AS builder

WORKDIR /osxcross

RUN git clone https://github.com/tpoechtrager/osxcross.git . \
  && git checkout 27d21e4977c9751d01199c7a226a6faf494c3dd9
# RUN UNATTENDED=1 ./build_apple_clang.sh
ENV MAC_SDK_VERSION 26.1
RUN curl -L --fail https://github.com/joseluisq/macosx-sdks/releases/download/$MAC_SDK_VERSION/MacOSX$MAC_SDK_VERSION.sdk.tar.xz -o /osxcross/tarballs/MacOSX$MAC_SDK_VERSION.sdk.tar.xz
# Use current LLVM tools; legacy cctools do not support LLVM 23's callback ABI.
RUN BUILD_FLAVOR=llvm UNATTENDED=1 ./build.sh
RUN cargo install apple-codesign
ENV PATH /osxcross/target/bin:$PATH

LABEL org.opencontainers.image.authors="support@stream.place"
