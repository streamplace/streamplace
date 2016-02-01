FROM node:5

# I'm designed to be run on the pratty-apps root directory. This app installs local packages from there.

WORKDIR /build
ADD . /build
RUN \
  mkdir /app &&  \
  cd /build/bellamie && npm install --production && \
  mv /build/bellamie/node_modules /app/node_modules && \
  rm -rf /build
ADD bellamie/dist /app/dist
ENV PORT 80
EXPOSE 80
CMD ["node", "/app/dist/app.js"]
