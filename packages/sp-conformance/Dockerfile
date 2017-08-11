
FROM streamplace/sp-ffmpeg

ADD package.json /app/package.json
ADD tests /app/tests
WORKDIR /app
RUN npm install

CMD npm run test
