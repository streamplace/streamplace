FROM node:6

# To be run from the apps directory.

WORKDIR /app
ADD bellamie/package.json /app/package.json
RUN npm install --production

ADD sk-schema/package.json /packages/sk-schema/package.json
ADD sk-schema/dist /packages/sk-schema/dist
RUN npm install --production /packages/sk-schema && rm -rf /packages

ADD bellamie/dist /app/dist

ENV PORT 80
EXPOSE 80
CMD ["node", "/app/dist/app.js"]
