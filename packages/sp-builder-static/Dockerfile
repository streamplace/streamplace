FROM stream.place/sp-node

ENV NODE_ENV development
ONBUILD ADD package.json /app/package.json
ONBUILD RUN npm install --no-scripts
ONBUILD COPY src /app/src
ONBUILD COPY public /app/public
