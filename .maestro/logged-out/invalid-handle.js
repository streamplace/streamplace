// Moves the harness account to a .test handle that public resolution can't
// verify, so the node reports the "handle.invalid" sentinel for it, or
// (INVALID_HANDLE_STEP=restore) back to ACCOUNT_HANDLE. Same DID, same live
// stream. Maestro runs this on the host, so it reaches the PDS and node over
// their plain-HTTP loopback listeners.
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

const restore = INVALID_HANDLE_STEP === "restore";
const handle = restore
  ? ACCOUNT_HANDLE
  : `invalid-${Date.now().toString(36)}.test`;
const { accessJwt } = json(
  post(`${PDS_HTTP_URL}/xrpc/com.atproto.server.createSession`, {
    identifier: ACCOUNT_DID,
    password: ACCOUNT_PASSWORD,
  }).body,
);
post(
  `${PDS_HTTP_URL}/xrpc/com.atproto.identity.updateHandle`,
  { handle },
  { Authorization: `Bearer ${accessJwt}` },
);
// The node re-reads the DID document and verifies the handle it claims.
const identity = json(
  post(`${SERVER_HTTP_URL}/xrpc/com.atproto.identity.refreshIdentity`, {
    identifier: ACCOUNT_DID,
  }).body,
);
const expected = restore ? ACCOUNT_HANDLE : "handle.invalid";
if (identity.handle !== expected) {
  throw new Error(
    `refreshIdentity after updateHandle(${handle}): handle ${identity.handle}, want ${expected}`,
  );
}
console.log(`${ACCOUNT_DID} now resolves to ${identity.handle}`);
