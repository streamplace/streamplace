FROM quay.io/streamplace/sp-builder AS builder
FROM quay.io/streamplace/sp-node

ADD package.json /app/package.json
RUN npm install --production
COPY --from=quay.io/streamplace/sp-client /app /app/node_modules/sp-client
COPY --from=quay.io/streamplace/sp-resource /app /app/node_modules/sp-resource
COPY --from=builder /app/dist /app/dist
