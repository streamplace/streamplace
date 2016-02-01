FROM node:5

# I'm designed to be run on the pratty-apps root directory. This app installs local packages from there.

ENV PORT 80
EXPOSE 80
WORKDIR /app
ADD lib /lib
ADD bellamie/package.json /app/package.json
RUN \
  npm install --production && \
  rm -rf /lib
ADD bellamie/dist /app/dist
CMD ["node", "/app/dist/app.js"]
