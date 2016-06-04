
FROM streamkitchen/sk-node

RUN \
  apt-get update && \
  apt-get install -y python make g++ libtool autoconf automake pkg-config curl && \
  apt-get install -y libvorbisenc2 libvorbis0a libtheora0 libfreetype6 libass5 && \
  apt-get install -y nasm && \
  rm -rf /var/lib/apt/lists/*

# We need a newer veresion of libzmq.
ADD get-zmq.sh /get-zmq.sh
RUN cd ~ && PREFIX="/usr" /get-zmq.sh && rm /get-zmq.sh

RUN curl -L -o /usr/bin/docker https://get.docker.com/builds/Linux/x86_64/docker-1.10.3 && \
  chmod 755 /usr/bin/docker
