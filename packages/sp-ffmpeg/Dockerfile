FROM stream.place/sp-node

RUN apt-get update \
  && apt-get install -y software-properties-common python-software-properties \
  && if test "$(dpkg --print-architecture)" = "amd64"; then add-apt-repository -y ppa:jonathonf/ffmpeg-3; fi \
  && apt-get update \
  && apt-get install -y ffmpeg

# also get a snapshot version, a couple features need it but it's less stable
RUN curl -LO https://johnvansickle.com/ffmpeg/builds/ffmpeg-git-64bit-static.tar.xz \
  && tar xvf ffmpeg-git-64bit-static.tar.xz \
  && rm ffmpeg-git-64bit-static.tar.xz \
  && mv ffmpeg*/ffmpeg /usr/bin/ffmpeg-unstable \
  && rm -rf ./ffmpeg*

CMD [ "ffmpeg" ]

