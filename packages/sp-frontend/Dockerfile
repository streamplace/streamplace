FROM stream.place/streamplace as builder
ENV NODE_ENV development
WORKDIR /app/node_modules/sp-frontend
ADD package.json package.json
ADD src src
ADD public public
RUN yarn install && npm run prepare

FROM stream.place/sp-static
COPY --from=builder /app/node_modules/sp-frontend/build /app/dist
