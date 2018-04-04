FROM quay.io/streamplace/sp-builder-static as builder
COPY --from=quay.io/streamplace/sp-styles /app /app/node_modules/sp-styles
COPY --from=quay.io/streamplace/sp-client /app /app/node_modules/sp-client
COPY --from=quay.io/streamplace/sp-components /app /app/node_modules/sp-components
COPY --from=quay.io/streamplace/streamplace-ui /app /app/node_modules/streamplace-ui
RUN npm run prepare && mv /app/build /app/dist
FROM quay.io/streamplace/sp-static

COPY --from=builder /app/dist /app/dist
