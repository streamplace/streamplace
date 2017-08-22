FROM quay.io/streamplace/sp-node

# To be run from the apps directory.

RUN npm install -g kubectl-cli --unsafe-perm

WORKDIR /app
ADD package.json package.json
RUN npm install --production --unsafe-perm
ADD dist dist

ENV PORT 80
EXPOSE 80
CMD node /app/dist/sp-api-server.js
