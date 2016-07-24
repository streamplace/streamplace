FROM streamkitchen/sk-node

WORKDIR /apps/vertex-scheduler
ADD package.json package.json
RUN npm install --production
ADD dist dist

CMD ["node", "dist/vertex-scheduler.js"]
