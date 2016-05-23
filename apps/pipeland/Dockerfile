FROM gcr.io/stream-kitchen/sk-node

# To be run from the apps directory.

# First line is libzmq dependencies, second is ffmpeg dependencies
RUN \
  apt-get update && \
  apt-get install -y python make g++ libtool autoconf automake pkg-config curl && \
  apt-get install -y libvorbisenc2 libvorbis0a libtheora0 libfreetype6 libass5 && \
  apt-get install -y nasm && \
  rm -rf /var/lib/apt/lists/*

# We need a newer veresion of libzmq.
RUN cd ~ && \
  git clone https://github.com/zeromq/libzmq && \
  cd libzmq && \
  git checkout 464d3fd3f8cf606c99c1a03806fcb3230314b90b && \
  ./autogen.sh && \
  ./configure --prefix="/usr" && \
  make -j 4 && \
  make install

RUN cd ~ && \
  curl -LO https://github.com/cisco/openh264/archive/v1.5.0.tar.gz && \
  tar xzvf v1.5.0.tar.gz && \
  cd openh264* && \
  make && \
  make install PREFIX="/usr"

WORKDIR /app
ADD sk-ffmpeg/bin/ffmpeg /usr/local/bin/ffmpeg
# For some reason FFProbe breaks listen ports. A fluent-ffmpeg bug problably. Anyway, gone for now.
# ADD sk-ffmpeg/bin/ffprobe /usr/local/bin/ffprobe

ADD pipeland/package.json /app/package.json

ADD mpeg-munger/package.json /packages/mpeg-munger/package.json
ADD mpeg-munger/dist /packages/mpeg-munger/dist

ADD sk-schema/package.json /packages/sk-schema/package.json
ADD sk-schema/dist /packages/sk-schema/dist

ADD sk-client/package.json /packages/sk-client/package.json
ADD sk-client/dist /packages/sk-client/dist

RUN npm install --production /packages/mpeg-munger && \
  npm install --production /packages/sk-client && \
  npm install --production /packages/sk-schema && \
  npm install --production && rm -rf /packages

ADD pipeland/dist /app/dist
ADD /run.sh /run.sh

CMD ["/run.sh"]
