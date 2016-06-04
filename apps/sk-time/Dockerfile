FROM streamkitchen/sk-node

WORKDIR /app
ADD package.json package.json
RUN npm install --production
ADD dist /app/dist

ENV PORT 80
EXPOSE 80
CMD ["node", "/app/dist/sk-time.js"]
