FROM stream.place/sp-node

WORKDIR /app
ENV NODE_ENV development
ONBUILD ADD package.json /app/package.json
ONBUILD RUN npm install --no-scripts
ONBUILD COPY src /app/src
ONBUILD RUN npm run prepare
