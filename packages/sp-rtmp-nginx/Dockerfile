FROM ubuntu:xenial AS builder

RUN apt-get update

RUN apt-get install -y curl gcc libpcre3 libpcre3-dev zlibc libssl-dev make

ENV NGINX_VER "1.10.1"
ENV NGINX_RTMP_VER "1.1.11"

WORKDIR /build

RUN curl -L -o nginx-rtmp.tar.gz https://github.com/arut/nginx-rtmp-module/archive/v$NGINX_RTMP_VER.tar.gz
RUN curl -L -o nginx.tar.gz http://nginx.org/download/nginx-$NGINX_VER.tar.gz
RUN tar xvzf nginx.tar.gz
RUN tar xvzf nginx-rtmp.tar.gz

WORKDIR /build/nginx-$NGINX_VER
RUN ./configure --add-module=/build/nginx-rtmp-module-$NGINX_RTMP_VER --with-http_ssl_module
RUN make && make install

FROM stream.place/sp-node

COPY --from=builder /usr/local/nginx/sbin/nginx /usr/bin/nginx
COPY nginx.conf /usr/local/nginx/conf/nginx.conf
CMD nginx
