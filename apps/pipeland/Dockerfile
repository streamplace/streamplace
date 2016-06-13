FROM streamkitchen/sk-node

# To be run from the apps directory.

# First line is libzmq dependencies, second is ffmpeg dependencies
RUN \
  apt-get update && \
  apt-get install -y python make g++ libtool autoconf automake pkg-config curl && \
  apt-get install -y libvorbisenc2 libvorbis0a libtheora0 libfreetype6 libass5 && \
  apt-get install -y nasm && \
  rm -rf /var/lib/apt/lists/*

# We need a newer veresion of libzmq.
ADD sk-builder/get-zmq.sh /get-zmq.sh
RUN cd ~ && PREFIX="/usr" /get-zmq.sh && rm /get-zmq.sh

WORKDIR /app

ADD pipeland/package.json /app/package.json

ADD mpeg-munger/package.json /packages/mpeg-munger/package.json
ADD mpeg-munger/dist /packages/mpeg-munger/dist

ADD sk-schema/package.json /packages/sk-schema/package.json
ADD sk-schema/dist /packages/sk-schema/dist

ADD sk-schema/package.json /packages/sk-schema/package.json
ADD sk-schema/dist /packages/sk-schema/dist

ADD sk-client/package.json /packages/sk-client/package.json
ADD sk-client/dist /packages/sk-client/dist

ADD sk-config/package.json /packages/sk-config/package.json
ADD sk-config/dist /packages/sk-config/dist

RUN npm install --production /packages/mpeg-munger && \
  npm install --production /packages/sk-client && \
  npm install --production /packages/sk-schema && \
  npm install --production /packages/sk-config && \
  npm install --production && rm -rf /packages

ADD pipeland/run.sh /run.sh
ADD pipeland/dist /app/dist

CMD ["/run.sh"]
