FROM streamplace/sp-node:latest

ADD package.json /app/package.json
RUN npm install -g --unsafe-perm kubectl-cli
RUN npm install --unsafe-perm
ADD dist /app/dist

CMD node /app/dist/sp-schema.js
