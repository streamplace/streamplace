FROM ubuntu:xenial
MAINTAINER Streamplace <stuff@stream.place>

ENV DEBIAN_FRONTEND noninteractive
ENV NWJS_VERSION v0.20.1
ENV NWJS_INSTALL_DIR /opt/nwjs

RUN apt-get update && \
  apt-get install build-essential ca-certificates xterm fluxbox eterm supervisor libexif12 libgconf-2-4 curl xvfb x11vnc -y  && \
  apt-get --reinstall install xfonts-base -y && \
  apt-get clean autoclean && \
  apt-get autoremove --yes && \
  rm -rf /var/lib/{apt,dpkg,cache,log}/

#install nwjs
RUN mkdir -p ${NWJS_INSTALL_DIR} && \
    curl -L http://dl.nwjs.io/${NWJS_VERSION}/nwjs-sdk-${NWJS_VERSION}-linux-x64.tar.gz | tar -xz --strip=1 -C ${NWJS_INSTALL_DIR} && \
    chown -R root:root ${NWJS_INSTALL_DIR}


ENV     DISPLAY :99
ENV     XVFB_SCREENSIZE 1920x1080x16
ENV     NWJS_ARGS ""

RUN apt-get update && apt-get install -y libnss3 libxcursor1 libcups2 libasound2 libpangocairo-1.0-0 libatk1.0-0 libgdk-pixbuf2.0-0 libgtk2.0-0 xserver-xorg-video-intel libgl1-mesa-dri  libglapi-mesa  libosmesa6  mesa-utils

ADD     supervisor/conf.d/ /etc/supervisor/conf.d/

EXPOSE 5900

ENV NWJS_ARGS "--enable-node-worker /app"
ADD . /app

WORKDIR /app

CMD    ["/usr/bin/supervisord"]
