FROM streamkitchen/sk-node

# To be run from the apps directory.

WORKDIR /apps/bellamie
ADD package.json package.json
RUN npm install --production
ADD dist dist

ENV PORT 80
EXPOSE 80
CMD ["node", "dist/app.js"]
