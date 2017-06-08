FROM quay.io/streamplace/sp-node

RUN \
  apt-get update && \
  apt-get install -y nginx && \
  rm -rf /var/lib/apt/lists/*

ADD nginx.conf /etc/nginx/nginx.conf

CMD ["nginx"]
