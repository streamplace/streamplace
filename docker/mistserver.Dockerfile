ARG TARGETARCH
FROM --platform=linux/$TARGETARCH ubuntu:24.04
RUN apt update && apt install -y curl
ARG STREAMPLACE_URL
ENV STREAMPLACE_URL $STREAMPLACE_URL
# strip the -cloudflare suffix from the url; we're on the git server we don't need to leave
RUN export LOCAL_URL="$(echo $STREAMPLACE_URL | sed 's/-cloudflare//')" && echo "downloading $LOCAL_URL" && cd /usr/local/bin && curl -L "$LOCAL_URL" | tar xzv

RUN apt-get update && apt-get install -y curl
RUN curl -o - https://releases.mistserver.org/is/mistserver_64V3.6.1.tar.gz 2>/dev/null | sh
RUN mkdir -p /config
ADD ./docker/mistserver.json /config/mistserver.json
CMD ["MistController", "-c", "/config/mistserver.json"]
