
FROM quay.io/streamplace/sp-node

ADD package.json /app/package.json
RUN npm install --production
ADD dist /app/dist
