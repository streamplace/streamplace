# amd64-only MistServer companion image published to ghcr.io by
# .github/workflows/docker.yaml. amd64 only because the upstream MistServer
# release (mistserver_64) is x86-64. The streamplace binary is baked in from the
# build context; see docker/release.ghcr.Dockerfile for the rationale.
FROM --platform=linux/amd64 ubuntu:24.04
RUN apt-get update && apt-get install -y curl ca-certificates && rm -rf /var/lib/apt/lists/*
COPY build-linux-amd64/streamplace /usr/local/bin/streamplace
RUN chmod +x /usr/local/bin/streamplace
RUN cd /usr/bin && curl -L -o - https://r.mistserver.org/dl/mistserver_64V3.7.tar.gz | tar xzv
RUN mkdir -p /config
ADD mistserver.json /config/mistserver.json
CMD ["MistController", "-c", "/config/mistserver.json"]
