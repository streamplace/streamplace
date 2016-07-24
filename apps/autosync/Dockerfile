FROM streamkitchen/sk-node

WORKDIR /apps/autosync
ADD package.json package.json
RUN npm install --production
ADD dist dist

CMD ["node", "dist/autosync.js"]
