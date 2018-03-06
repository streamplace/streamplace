FROM quay.io/streamplace/sp-ffmpeg

ENV NODE_ENV development
WORKDIR /app
ADD package.json /app/package.json
RUN npm install
ADD __test__ /app/__test__
ADD src /app/src

RUN npm run test-in-container
