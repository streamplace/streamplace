FROM quay.io/streamplace/sp-node as builder

ENV NODE_ENV development
WORKDIR /app
ADD package.json /app/package.json
RUN npm install --ignore-scripts
ADD src /app/src
RUN npm run build

FROM quay.io/streamplace/sp-node

WORKDIR /app
ADD package.json /app/package.json
RUN npm install
COPY --from=builder /app/dist /app/dist

CMD node /app/dist/sp-redirects.js
