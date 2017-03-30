FROM streamplace/sp-node:latest

ADD package.json /app/package.json
RUN npm install -g kubectl-cli
RUN npm install
ADD dist /app/dist

CMD node /app/dist/sp-schema.js
