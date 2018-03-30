FROM quay.io/streamplace/sp-builder AS builder
FROM quay.io/streamplace/sp-ffmpeg

WORKDIR /app
ADD package.json /app/package.json
RUN npm install --production
COPY --from=builder /app/dist /app/dist

CMD ["node", "/app/dist/sp-rtmp-server.js"]
