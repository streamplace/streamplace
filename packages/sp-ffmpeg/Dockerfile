FROM quay.io/streamplace/sp-node

RUN apt-get update \
  && apt-get install -y software-properties-common python-software-properties \
  && add-apt-repository -y ppa:jonathonf/ffmpeg-3 \
  && apt-get update \
  && apt-get install -y ffmpeg

CMD [ "ffmpeg" ]
