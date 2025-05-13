FROM ubuntu:24.04

RUN apt-get update && apt-get install -y curl
RUN curl -o - https://releases.mistserver.org/is/mistserver_64V3.6.1.tar.gz 2>/dev/null | sh
ENV PATH "$PATH:/usr/local/bin"
CMD ["MistController", "-c", "/config/mistserver.json"]
