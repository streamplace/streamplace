FROM node:5

# I'm designed to be run on the pratty-apps root directory. This app installs local packages from there.

# Copied from sk-ffmpeg. Docker really needs to allow composing Dockerfiles, that'd be rad.
RUN \
  echo "deb http://www.deb-multimedia.org jessie main non-free" >> /etc/apt/sources.list && \
  echo "deb-src http://www.deb-multimedia.org jessie main non-free" >> /etc/apt/sources.list && \
  apt-get update && apt-get install -y --force-yes \
  libvorbisenc2 libtheora0 librtmp1 libopencore-amrwb0 libopencore-amrnb0 libmp3lame0 libfaac0
ADD sk-ffmpeg/build/ffmpeg /usr/bin/ffmpeg
ADD sk-ffmpeg/build/ffserver /usr/bin/ffserver
ADD sk-ffmpeg/build/ffprobe /usr/bin/ffprobe

WORKDIR /app/build
ADD lib /app/lib
ADD pipeland/package.json /app/build/package.json
RUN \
  npm install --production
ADD pipeland/dist /app/build/dist
CMD ["node", "/app/build/dist/app.js"]
