FROM quay.io/streamplace/sp-builder AS builder
FROM quay.io/streamplace/sp-ffmpeg

COPY --from=builder /app/dist /app/dist

CMD ["node", "/app/dist/sp-broadcaster.js"]
