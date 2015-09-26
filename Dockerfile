FROM debian:jessie

RUN apt-get update

RUN apt-get install -y curl gcc libpcre3 libpcre3-dev zlibc libssl-dev make

WORKDIR /build

RUN curl -L -o nginx-rtmp.tar.gz https://github.com/arut/nginx-rtmp-module/archive/v1.1.7.tar.gz
RUN curl -L -o nginx.tar.gz http://nginx.org/download/nginx-1.9.5.tar.gz
RUN tar xvzf nginx.tar.gz
RUN tar xvzf nginx-rtmp.tar.gz

WORKDIR /build/nginx-1.9.5
RUN ./configure
RUN make
RUN make install

EXPOSE 80
EXPOSE 443

ADD nginx.conf /usr/local/nginx/conf/nginx.conf

CMD /usr/local/nginx/sbin/nginx