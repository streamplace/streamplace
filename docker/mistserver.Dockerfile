ARG TARGETARCH
FROM --platform=linux/$TARGETARCH ubuntu:24.04
RUN apt update && apt install -y curl
ARG STREAMPLACE_URL
ENV STREAMPLACE_URL $STREAMPLACE_URL
# strip the -cloudflare suffix from the url; we're on the git server we don't need to leave
RUN export LOCAL_URL="$(echo $STREAMPLACE_URL | sed 's/-cloudflare//')" && echo "downloading $LOCAL_URL" && cd /usr/local/bin && curl -L "$LOCAL_URL" | tar xzv

RUN apt-get update && apt-get install -y curl
# Download to a file with retries, then extract (see mistserver.ghcr.Dockerfile):
# piping curl into tar corrupts extraction on a truncated transfer, and
# r.mistserver.org hiccups intermittently.
RUN cd /usr/bin && \
    curl -fL --retry 12 --retry-all-errors --retry-delay 10 --retry-max-time 240 -o /tmp/mistserver.tar.gz https://storage.googleapis.com/streamplace-crap/6619b22c-7744-4be4-9de8-cdbf22ce5906/MistServer.tar.gz && \
    tar xzvf /tmp/mistserver.tar.gz && \
    rm /tmp/mistserver.tar.gz
RUN mkdir -p /config
ADD ./docker/mistserver.json /config/mistserver.json
CMD ["MistController", "-c", "/config/mistserver.json"]
