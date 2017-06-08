
FROM quay.io/streamplace/sp-node

WORKDIR /app
ADD package.json /app/package.json
RUN npm install --production
ADD dist /app/dist

CMD exec node /app/dist/sp-dev-certs.js
