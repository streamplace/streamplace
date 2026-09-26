// Whether the node serves BRAND_TITLE on the custom domain yet: it follows
// the record off the firehose, a moment after publish-brand.js.
const res = http.get(
  CUSTOM_DOMAIN_URL + "/xrpc/place.stream.branding.getBranding",
);
output.brandSynced =
  res.ok &&
  json(res.body).assets.some(
    (a) => a.key === "siteTitle" && a.data === BRAND_TITLE,
  );
