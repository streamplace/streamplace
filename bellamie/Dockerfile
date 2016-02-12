FROM node:5

# I'm designed to be run on the pratty-apps root directory. This app installs local packages from there.

ENV PORT 80
EXPOSE 80
WORKDIR /app
ADD lib /app/lib
ADD bellamie/package.json /app/build/package.json
RUN \
  cd /app/build && \
  npm install --production && \
  cd /app && \
  mv /app/build/node_modules /app/node_modules && \
  rm -rf /app/build
ADD bellamie/dist /app/dist
CMD ["node", "/app/dist/app.js"]
