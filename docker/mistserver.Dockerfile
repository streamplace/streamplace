ARG TARGETARCH
FROM --platform=linux/$TARGETARCH ubuntu:24.04
RUN apt update && apt install -y curl
ARG STREAMPLACE_URL
ENV STREAMPLACE_URL $STREAMPLACE_URL
# strip the -cloudflare suffix from the url; we're on the git server we don't need to leave
RUN export LOCAL_URL="$(echo $STREAMPLACE_URL | sed 's/-cloudflare//')" && echo "downloading $LOCAL_URL" && cd /usr/local/bin && curl -L "$LOCAL_URL" | tar xzv

RUN apt-get update && apt-get install -y curl
RUN cd /usr/bin && curl -o - https://r.mistserver.org/dl/mistserver_64V3.7.tar.gz | tar xzv
RUN mkdir -p /config
ADD ./docker/mistserver.json /config/mistserver.json
CMD ["MistController", "-c", "/config/mistserver.json"]
