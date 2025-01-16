
ARG TARGETARCH
FROM --platform=linux/$TARGETARCH ubuntu:24.04
RUN apt update && apt install -y curl
ARG STREAMPLACE_URL
ENV STREAMPLACE_URL $STREAMPLACE_URL
RUN echo "downloading $STREAMPLACE_URL" && cd /usr/local/bin && curl -L "$STREAMPLACE_URL" | tar xzv
RUN streamplace self-test
CMD streamplace
