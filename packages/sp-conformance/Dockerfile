
FROM quay.io/streamplace/sp-ffmpeg

ADD . /app
WORKDIR /app
ENV SP_VALUES_FILE /values.yaml
RUN npm install

CMD npm run test
