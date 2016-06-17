FROM streamkitchen/sk-node

RUN \
  apt-get update && \
  apt-get install -y nginx && \
  rm -rf /var/lib/apt/lists/* && \
  chown -R www-data:www-data /var/lib/nginx

ADD sk-config /packages/sk-config
RUN npm install --production -g /packages/sk-config && rm -rf /packages

ADD sk-static/nginx.conf /etc/nginx/nginx.conf
ADD sk-static/run.sh /run.sh

CMD ["/run.sh"]
