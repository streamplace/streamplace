FROM quay.io/streamplace/sp-builder as builder
FROM quay.io/streamplace/sp-node

ADD package.json package.json
RUN npm install --production --unsafe-perm
COPY --from=builder /app/dist dist

ENV PORT 80
EXPOSE 80
CMD node /app/dist/sp-api-server.js
