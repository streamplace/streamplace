# amd64-only MistServer companion image published to ghcr.io by
# .github/workflows/docker.yaml. amd64 only because the upstream MistServer
# release (mistserver_64) is x86-64. The streamplace binary is baked in from the
# build context; see docker/release.ghcr.Dockerfile for the rationale.
FROM --platform=linux/amd64 ubuntu:24.04
RUN apt-get update && apt-get install -y curl ca-certificates && rm -rf /var/lib/apt/lists/*
COPY build-linux-amd64/streamplace /usr/local/bin/streamplace
RUN chmod +x /usr/local/bin/streamplace
# Download to a file with retries, then extract. Piping curl straight into tar
# corrupts the extraction when the transfer is truncated, and r.mistserver.org
# hiccups intermittently (flaked CI repeatedly), so retry transient failures and
# only untar a complete download.
RUN cd /usr/bin && \
    curl -fL --retry 12 --retry-all-errors --retry-delay 10 --retry-max-time 240 -o /tmp/mistserver.tar.gz https://storage.googleapis.com/streamplace-crap/6619b22c-7744-4be4-9de8-cdbf22ce5906/MistServer.tar.gz && \
    tar xzvf /tmp/mistserver.tar.gz && \
    rm /tmp/mistserver.tar.gz
RUN mkdir -p /config
ADD mistserver.json /config/mistserver.json
CMD ["MistController", "-c", "/config/mistserver.json"]
