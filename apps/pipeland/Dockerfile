FROM streamkitchen/sk-ffmpeg

# To be run from the apps directory.

RUN \
  apt-get update && \
  apt-get install -y curl xz-utils git && \
  rm -rf /var/lib/apt/lists/*

# Stolen from https://github.com/nodejs/docker-node
# gpg keys listed at https://github.com/nodejs/node
RUN set -ex \
  && for key in \
    9554F04D7259F04124DE6B476D5A82AC7E37093B \
    94AE36675C464D64BAFA68DD7434390BDBE9B9C5 \
    0034A06D9D9B0064CE8ADF6BF1747F4AD2306D93 \
    FD3A5288F042B6850C66B31F09FE44734EB7990E \
    71DCFD284A79C3B38668286BC97EC7A07EDE3FC1 \
    DD8F2338BAE7501E3DD5AC78C273792F7D83545D \
    B9AE9905FFD7803F25714661B63B535A4C206CA9 \
    C4F0DFFF4E8C1A8236409D08E73BC641CC11F4C8 \
  ; do \
    gpg --keyserver ha.pool.sks-keyservers.net --recv-keys "$key"; \
  done

ENV NPM_CONFIG_LOGLEVEL warn
ENV NODE_VERSION 6.2.1

RUN curl -SLO "https://nodejs.org/dist/v$NODE_VERSION/node-v$NODE_VERSION-linux-x64.tar.xz" \
  && curl -SLO "https://nodejs.org/dist/v$NODE_VERSION/SHASUMS256.txt.asc" \
  && gpg --batch --decrypt --output SHASUMS256.txt SHASUMS256.txt.asc \
  && grep " node-v$NODE_VERSION-linux-x64.tar.xz\$" SHASUMS256.txt | sha256sum -c - \
  && tar -xJf "node-v$NODE_VERSION-linux-x64.tar.xz" -C /usr/local --strip-components=1 \
  && rm "node-v$NODE_VERSION-linux-x64.tar.xz" SHASUMS256.txt.asc SHASUMS256.txt

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

ENV SK_CONFIG /config.yml

CMD ["node", "/app/dist/app.js"]
