FROM quay.io/streamplace/sp-builder AS builder
FROM quay.io/streamplace/sp-node

COPY --from=quay.io/streamplace/sp-client /app /app/node_modules/sp-client
COPY --from=builder /app/dist /app/dist

CMD ["node", "/app/dist/sp-channel-manager.js"]
