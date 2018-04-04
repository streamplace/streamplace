
FROM quay.io/streamplace/sp-ffmpeg

ADD . /app
WORKDIR /app
ENV SP_VALUES_FILE /values.yaml
RUN npm install
COPY --from=quay.io/streamplace/sp-client /app /app/node_modules/sp-client
CMD npm run conformance-tests
