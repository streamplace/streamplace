FROM quay.io/streamplace/sp-builder-static as builder
FROM quay.io/streamplace/sp-static

COPY --from=builder /app/dist /app/dist
