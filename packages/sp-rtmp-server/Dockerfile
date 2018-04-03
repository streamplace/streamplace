FROM quay.io/streamplace/sp-builder AS builder
FROM quay.io/streamplace/sp-ffmpeg

WORKDIR /app
ADD package.json /app/package.json
RUN npm install --production
COPY --from=quay.io/streamplace/sp-client /app /app/node_modules/sp-client
COPY --from=builder /app/dist /app/dist

CMD ["node", "/app/dist/sp-rtmp-server.js"]
