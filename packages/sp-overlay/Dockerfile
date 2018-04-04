FROM quay.io/streamplace/sp-builder-static as builder
RUN npm run prepare && mv /app/build /app/dist
FROM quay.io/streamplace/sp-static

COPY --from=builder /app/dist /app/dist
