// Publish a brand for the harness's custom domain the way any atproto
// client could: a password session on the test account's PDS, then its
// place.stream.branding.brand record keyed by the domain's hostname. Runs
// on the Maestro host, so the harness URLs need no device rewriting.
const host = CUSTOM_DOMAIN_URL.replace(/^https?:\/\//, "").replace(
  /[:/].*$/,
  "",
);
const session = json(
  http.post(PDS_URL + "/xrpc/com.atproto.server.createSession", {
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({
      identifier: ACCOUNT_HANDLE,
      password: ACCOUNT_PASSWORD,
    }),
  }).body,
);
const res = http.post(PDS_URL + "/xrpc/com.atproto.repo.putRecord", {
  headers: {
    "Content-Type": "application/json",
    Authorization: "Bearer " + session.accessJwt,
  },
  body: JSON.stringify({
    repo: ACCOUNT_DID,
    collection: "place.stream.branding.brand",
    rkey: host,
    record: {
      $type: "place.stream.branding.brand",
      siteTitle: BRAND_TITLE,
      primaryColor: "#e11d48",
    },
  }),
});
if (!res.ok) {
  throw new Error("putRecord failed: " + res.status + " " + res.body);
}
output.brandSynced = false;
