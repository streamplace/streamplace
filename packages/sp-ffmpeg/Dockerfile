FROM quay.io/streamplace/sp-node

RUN curl -LO https://johnvansickle.com/ffmpeg/builds/ffmpeg-git-64bit-static.tar.xz \
  && tar xvf ffmpeg-git-64bit-static.tar.xz \
  && rm ffmpeg-git-64bit-static.tar.xz \
  && mv ffmpeg*/ffmpeg /usr/bin/ffmpeg \
  && rm -rf ./ffmpeg*

CMD [ "ffmpeg" ]
