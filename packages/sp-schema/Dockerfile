FROM quay.io/streamplace/sp-builder AS builder
FROM quay.io/streamplace/sp-node

COPY --from=builder /app/dist /app/dist

CMD node /app/dist/sp-schema.js
