
FROM streamplace/sp-ffmpeg

WORKDIR /app
ADD package.json /app/package.json
RUN npm install --production
ADD dist /app/dist

CMD ["node", "/app/dist/sp-broadcaster.js"]
