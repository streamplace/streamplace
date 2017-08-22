FROM quay.io/streamplace/sp-node:latest

ADD package.json /app/package.json
RUN npm install -g kubectl-cli --unsafe-perm
RUN npm install --unsafe-perm
ADD dist /app/dist

CMD node /app/dist/sp-schema.js
