FROM quay.io/streamplace/sp-builder as builder
FROM quay.io/streamplace/sp-node

ENV PORT 80
EXPOSE 80

ADD package.json package.json
RUN npm install --production --unsafe-perm
COPY --from=quay.io/streamplace/sp-client /app /app/node_modules/sp-client
COPY --from=quay.io/streamplace/sp-resource /app /app/node_modules/sp-resource
COPY --from=builder /app/dist dist

CMD node /app/dist/sp-api-server.js
