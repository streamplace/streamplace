FROM quay.io/streamplace/sp-node

WORKDIR /app
ENV NODE_ENV development
ONBUILD ADD package.json /app/package.json
ONBUILD RUN npm install
ONBUILD ADD src /app/src
ONBUILD RUN npm run prepare
