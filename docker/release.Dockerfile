
ARG TARGETARCH
FROM --platform=linux/$TARGETARCH ubuntu:24.04
RUN apt update && apt install -y curl
ARG STREAMPLACE_URL
ENV STREAMPLACE_URL $STREAMPLACE_URL
# strip the -cloudflare suffix from the url; we're on the git server we don't need to leave
RUN export LOCAL_URL="$(echo $STREAMPLACE_URL | sed 's/-cloudflare//')" && echo "downloading $LOCAL_URL" && cd /usr/local/bin && curl -L "$LOCAL_URL" | tar xzv
RUN streamplace self-test
ENV SP_DATA_DIR=/var/lib/streamplace
CMD streamplace
