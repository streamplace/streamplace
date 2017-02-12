
FROM streamplace/sp-node:latest

ENV HELM_VERSION v2.1.3

RUN apt-get update && \
  apt-get install -y jq realpath curl && \
  curl -o helm.tar.gz "https://storage.googleapis.com/kubernetes-helm/helm-$HELM_VERSION-linux-amd64.tar.gz" && \
  tar xzvf helm.tar.gz && \
  mv linux-amd64/helm /usr/bin/helm && \
  chmod 755 /usr/bin/helm && \
  rm -rf linux-amd64

ENV DOCKER_VERSION 1.10.3

RUN set -x && curl -L -o /root/docker.tgz "https://get.docker.com/builds/Linux/x86_64/docker-$DOCKER_VERSION.tgz" && \
  (cd / && tar xzvf /root/docker.tgz)
