FROM node:5

WORKDIR /app
ADD package.json /app/package.json
RUN npm install --production
ADD lib /app/lib
EXPOSE 80
CMD ["node", "/app/lib/app.js"]
