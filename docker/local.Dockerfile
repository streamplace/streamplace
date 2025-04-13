FROM ubuntu:24.04

RUN apt update && apt install -y ca-certificates

COPY build-linux-amd64/streamplace /usr/local/bin/streamplace

ENV PATH="/usr/local/bin:$PATH"

CMD ["streamplace"]
