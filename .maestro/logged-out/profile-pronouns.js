// Writes whitespace-padded pronouns into the harness account's real
// app.bsky.actor.profile record on the PDS (PROFILE_PRONOUNS_STEP=set),
// or restores the original record (restore). An originally missing profile
// becomes an empty record: the node indexes updates, but not deletions.
// Maestro runs this on the host, so it reaches the PDS over its plain-HTTP
// loopback listener.
function post(url, body, headers) {
  const res = http.post(url, {
    headers: Object.assign({ "Content-Type": "application/json" }, headers),
    body: JSON.stringify(body),
  });
  if (!res.ok) {
    throw new Error(`${url}: HTTP ${res.status}: ${res.body}`);
  }
  return res;
}

function put(record) {
  const { accessJwt } = json(
    post(`${PDS_HTTP_URL}/xrpc/com.atproto.server.createSession`, {
      identifier: ACCOUNT_DID,
      password: ACCOUNT_PASSWORD,
    }).body,
  );
  post(
    `${PDS_HTTP_URL}/xrpc/com.atproto.repo.putRecord`,
    {
      repo: ACCOUNT_DID,
      collection: "app.bsky.actor.profile",
      rkey: "self",
      record,
    },
    { Authorization: `Bearer ${accessJwt}` },
  );
  console.log(`${ACCOUNT_DID} profile: ${JSON.stringify(record)}`);
}

const got = http.get(
  `${PDS_HTTP_URL}/xrpc/com.atproto.repo.getRecord?repo=${encodeURIComponent(ACCOUNT_DID)}&collection=app.bsky.actor.profile&rkey=self`,
);
let record = null;
if (got.ok) {
  record = json(got.body).value;
} else if (json(got.body).error !== "RecordNotFound") {
  throw new Error(`getRecord: HTTP ${got.status}: ${got.body}`);
}

if (PROFILE_PRONOUNS_STEP === "restore") {
  if (output.originalPronounsProfile) {
    put(JSON.parse(output.originalPronounsProfile));
  }
} else {
  output.originalPronounsProfile = JSON.stringify(
    record || { $type: "app.bsky.actor.profile" },
  );
  // profile-pronouns.yaml expects the app to show this trimmed.
  put(
    Object.assign(record || { $type: "app.bsky.actor.profile" }, {
      pronouns: "  ze/zir  ",
    }),
  );
}
