FROM streamkitchen/sk-node

# To be run from the apps directory.

WORKDIR /app
ADD sk-plugin-autosync/package.json /app/package.json
RUN npm install --production

ADD sk-schema/package.json /packages/sk-schema/package.json
ADD sk-schema/dist /packages/sk-schema/dist
RUN npm install --production /packages/sk-schema

ADD sk-config/package.json /packages/sk-config/package.json
ADD sk-config/dist /packages/sk-config/dist
RUN npm install --production /packages/sk-config

ADD sk-client/package.json /packages/sk-client/package.json
ADD sk-client/dist /packages/sk-client/dist
RUN npm install --production /packages/sk-client

RUN rm -rf /packages

ADD sk-plugin-autosync/dist /app/dist

CMD ["node", "/app/dist/sk-plugin-autosync.js"]
