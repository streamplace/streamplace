
FROM quay.io/streamplace/sp-node

ENV DOCKER_VERSION 1.12.5
WORKDIR /app

RUN curl -L -o /root/docker.tgz https://get.docker.com/builds/Linux/x86_64/docker-$DOCKER_VERSION.tgz && \
  (cd /root && tar xzvf /root/docker.tgz) && \
  mv /root/docker/docker /usr/bin/docker && \
  rm -rf /root/docker /root/docker.tgz

ADD package.json /app/package.json
RUN yarn install --production --pure-lockfile
ADD dist /app/dist
