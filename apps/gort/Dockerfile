FROM ubuntu:xenial

# Stolen from https://github.com/dockerfile/nginx/blob/master/Dockerfile
RUN \
  apt-get update && \
  apt-get install -y nginx && \
  rm -rf /var/lib/apt/lists/* && \
  echo "\ndaemon off;" >> /etc/nginx/nginx.conf && \
  chown -R www-data:www-data /var/lib/nginx

ADD preview.html /var/www/html/preview.html
ADD nginx.conf /etc/nginx/nginx.conf
ADD dist /var/www/html/dist
ADD entrypoint.sh /entrypoint.sh
CMD ["/entrypoint.sh", "/var/www/html/index.html"]
