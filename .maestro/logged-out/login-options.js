// Turns on every sign-in option in the node's branding, with the harness PDS
// as the entryway, or (LOGIN_OPTIONS_STEP=restore) clears them again. Maestro
// runs this on the host, so it reaches the PDS and node over their plain-HTTP
// loopback listeners, and authenticates to the node as the harness account
// (a node admin) with a PDS service-auth token.
function call(method, url, body, headers) {
  const res = http.request(url, {
    method,
    headers: Object.assign({ "Content-Type": "application/json" }, headers),
    body: body === undefined ? undefined : JSON.stringify(body),
  });
  if (!res.ok) {
    throw new Error(`${url}: HTTP ${res.status}: ${res.body}`);
  }
  return res.body ? json(res.body) : {};
}

// Maestro's script engine has no btoa; every value here is ASCII.
function base64(s) {
  const abc =
    "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/";
  let out = "";
  for (let i = 0; i < s.length; i += 3) {
    const n =
      (s.charCodeAt(i) << 16) |
      ((s.charCodeAt(i + 1) || 0) << 8) |
      (s.charCodeAt(i + 2) || 0);
    out += abc[(n >> 18) & 63] + abc[(n >> 12) & 63];
    out += i + 1 < s.length ? abc[(n >> 6) & 63] : "=";
    out += i + 2 < s.length ? abc[n & 63] : "=";
  }
  return out;
}

const { accessJwt } = call(
  "POST",
  `${PDS_HTTP_URL}/xrpc/com.atproto.server.createSession`,
  { identifier: ACCOUNT_DID, password: ACCOUNT_PASSWORD },
);
const { server } = call(
  "GET",
  `${SERVER_HTTP_URL}/xrpc/place.stream.broadcast.getBroadcaster`,
);

function branding(lxm, body) {
  const { token } = call(
    "GET",
    `${PDS_HTTP_URL}/xrpc/com.atproto.server.getServiceAuth?aud=${encodeURIComponent(server)}&lxm=${lxm}`,
    undefined,
    { Authorization: `Bearer ${accessJwt}` },
  );
  call("POST", `${SERVER_HTTP_URL}/xrpc/${lxm}`, body, {
    Authorization: `Bearer ${token}`,
  });
}

const values = {
  loginEntryway: PDS_URL,
  loginMethods: JSON.stringify(["apple", "google", "ethereum", "qr"]),
  walletConnectProjectId: "e2e-project",
};
for (const key of Object.keys(values)) {
  if (LOGIN_OPTIONS_STEP === "restore") {
    branding("place.stream.branding.deleteBlob", { key });
  } else {
    branding("place.stream.branding.updateBlob", {
      key,
      mimeType: "text/plain",
      data: base64(values[key]),
    });
  }
}
console.log(`login options ${LOGIN_OPTIONS_STEP}d`);
