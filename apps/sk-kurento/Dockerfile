FROM ubuntu:14.04

RUN apt-get update && apt-get -y dist-upgrade && apt-get -y install wget

RUN echo "deb http://ubuntu.kurento.org trusty kms6" > /etc/apt/sources.list.d/kurento.list && \
  wget -O - http://ubuntu.kurento.org/kurento.gpg.key |  apt-key add - && \
  apt-get update && \
  apt-get install -y kurento-media-server-6.0

ENV GST_DEBUG=Kurento*:5

CMD /usr/bin/kurento-media-server
