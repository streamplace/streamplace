FROM quay.io/streamplace/sp-builder-static as builder
COPY --from=quay.io/streamplace/sp-client /app /app/node_modules/sp-client
COPY --from=quay.io/streamplace/sp-styles /app /app/node_modules/sp-styles
RUN npm run prepare && mv /app/build /app/dist
FROM quay.io/streamplace/sp-static

COPY --from=builder /app/dist /app/dist
