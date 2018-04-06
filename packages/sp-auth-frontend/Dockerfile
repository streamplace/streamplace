FROM stream.place/streamplace as builder
ENV NODE_ENV development
WORKDIR /app/node_modules/sp-auth-frontend
ADD package.json package.json
ADD src src
ADD public public
RUN npm install && npm run prepare

FROM stream.place/sp-static
COPY --from=builder /app/node_modules/sp-auth-frontend/build /app/dist
