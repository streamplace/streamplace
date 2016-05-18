FROM ubuntu:xenial

RUN \
  apt-get update && \
  apt-get install -y nginx && \
  rm -rf /var/lib/apt/lists/* && \
  chown -R www-data:www-data /var/lib/nginx

ADD nginx.conf /etc/nginx/nginx.conf
ADD bin/musta /usr/bin/musta
ADD run.sh /run.sh
RUN mkdir /webs

CMD ["/run.sh"]
