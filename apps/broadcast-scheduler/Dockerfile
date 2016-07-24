FROM streamkitchen/sk-node

WORKDIR /apps/broadcast-scheduler
ADD package.json package.json
RUN npm install --production
ADD dist dist

CMD ["node", "dist/broadcast-scheduler.js"]
