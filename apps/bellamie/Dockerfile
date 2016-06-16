FROM streamkitchen/sk-node

# To be run from the apps directory.

WORKDIR /app
ADD bellamie/package.json /app/package.json
RUN npm install --production

ADD sk-schema/package.json /packages/sk-schema/package.json
ADD sk-schema/dist /packages/sk-schema/dist
RUN npm install --production /packages/sk-schema

ADD sk-config/package.json /packages/sk-config/package.json
ADD sk-config/dist /packages/sk-config/dist
RUN npm install --production /packages/sk-config

RUN rm -rf /packages

ADD bellamie/dist /app/dist

ENV PORT 80
EXPOSE 80
CMD ["node", "/app/dist/app.js"]
