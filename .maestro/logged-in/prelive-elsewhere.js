// Starts a second owner playback session for the pre-live account and uses
// it, so the node evicts the app's. Maestro runs this on the host: the PDS
// and node answer on their plain-HTTP loopback listeners, and the node sees
// 127.0.0.1 for this script and, through the HTTPS front, for the emulator.
function get(url, headers) {
  const res = http.get(url, { headers });
  if (!res.ok) {
    throw new Error(`${url}: HTTP ${res.status}: ${res.body}`);
  }
  return res;
}

const { accessJwt } = json(
  http.post(`${PDS_HTTP_URL}/xrpc/com.atproto.server.createSession`, {
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({
      identifier: PRELIVE_ACCOUNT_DID,
      password: PRELIVE_ACCOUNT_PASSWORD,
    }),
  }).body,
);
// The node takes the PDS's service-auth token for the method, not the
// PDS's own access token.
const nodeDid = json(get(`${SERVER_HTTP_URL}/.well-known/did.json`).body).id;
const lxm = "place.stream.playback.getPlaybackSession";
const { token } = json(
  get(
    `${PDS_HTTP_URL}/xrpc/com.atproto.server.getServiceAuth?aud=${encodeURIComponent(nodeDid)}&lxm=${lxm}`,
    { Authorization: `Bearer ${accessJwt}` },
  ).body,
);
const { sid } = json(
  get(`${SERVER_HTTP_URL}/xrpc/${lxm}`, { Authorization: `Bearer ${token}` })
    .body,
);
// Minting alone evicts nobody; the first playback request with the new sid does.
get(
  `${SERVER_HTTP_URL}/xrpc/place.stream.playback.getLivePlaylist?streamer=${encodeURIComponent(PRELIVE_ACCOUNT_DID)}&sid=${encodeURIComponent(sid)}`,
);
console.log(`${PRELIVE_ACCOUNT_DID} now plays from session ${sid}`);
