FROM streamkitchen/sk-node

RUN \
  apt-get update && \
  apt-get install -y nginx && \
  rm -rf /var/lib/apt/lists/* && \
  chown -R www-data:www-data /var/lib/nginx

ADD nginx.conf /etc/nginx/nginx.conf
ADD run.sh /run.sh

CMD ["/run.sh"]
