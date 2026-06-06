# Multi-arch release image published to ghcr.io by .github/workflows/docker.yaml.
#
# Unlike docker/release.Dockerfile (which curls a prebuilt tarball from a
# package registry), the streamplace binary is baked in from the build context
# that the workflow just cross-compiled, so there is no external download
# dependency. buildx populates TARGETARCH per platform, selecting the matching
# binary that the workflow staged under build-linux-<arch>/.
ARG TARGETARCH
FROM --platform=linux/$TARGETARCH ubuntu:24.04
RUN apt-get update && apt-get install -y curl ca-certificates && rm -rf /var/lib/apt/lists/*
ARG TARGETARCH
ARG BUILDARCH
COPY build-linux-${TARGETARCH}/streamplace /usr/local/bin/streamplace
# upload-artifact drops the executable bit, so restore it. Only self-test on the
# native arch — under QEMU emulation (e.g. arm64 built on an amd64 runner) the
# self-test is slow and flaky, so skip it there.
RUN chmod +x /usr/local/bin/streamplace \
  && if [ "$TARGETARCH" = "$BUILDARCH" ]; then \
       streamplace self-test; \
     else \
       echo "skipping self-test: $TARGETARCH image built under emulation on $BUILDARCH"; \
     fi
ENV SP_DATA_DIR=/var/lib/streamplace
CMD ["streamplace"]
