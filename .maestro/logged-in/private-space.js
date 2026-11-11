// Maestro runs on the host: read the PDS's loopback admin API, not device HTTPS.
if (PDS_KIND !== "spaces" && PDS_KIND !== "vlpds") {
  throw new Error(`private-space admin read is not supported for ${PDS_KIND}`);
}
const space = `at://${ACCOUNT_DID}/space/place.stream.personal/self`;
const method =
  PDS_KIND === "vlpds"
    ? "vlpds.admin.listSpaceRecords"
    : "com.atproto.space.listRecords";
// Local fixture credentials: admin:dev-admin-token / constants.ts admin:admin-pass.
const authorization =
  PDS_KIND === "vlpds"
    ? "Basic YWRtaW46ZGV2LWFkbWluLXRva2Vu"
    : "Basic YWRtaW46YWRtaW4tcGFzcw==";
let cursor;
const records = [];
do {
  const res = http.get(
    `${PDS_HTTP_URL}/xrpc/${method}?space=${encodeURIComponent(space)}&repo=${encodeURIComponent(ACCOUNT_DID)}&collection=place.stream.multistream.target&limit=100${cursor ? `&cursor=${encodeURIComponent(cursor)}` : ""}`,
    { headers: { Authorization: authorization } },
  );
  if (!res.ok) {
    throw new Error(`${method}: HTTP ${res.status}: ${res.body}`);
  }
  const body = json(res.body);
  records.push(...body.records);
  cursor = body.cursor;
} while (cursor);
const record = records.find(
  (r) =>
    r.value.name === output.privateSpaceName ||
    (output.privateSpaceRkey &&
      (r.rkey || r.uri?.split("/").pop()) === output.privateSpaceRkey),
);
if (PRIVATE_SPACE_STEP === "present") {
  if (!record) {
    throw new Error(`no private target named "${output.privateSpaceName}"`);
  }
  if (record.value.url !== output.privateSpaceUrl) {
    throw new Error(
      `unexpected private target: ${JSON.stringify(record.value)}`,
    );
  }
  output.privateSpaceRkey = record.rkey || record.uri?.split("/").pop();
  if (!output.privateSpaceRkey) {
    throw new Error(`private target has no rkey: ${JSON.stringify(record)}`);
  }
} else if (PRIVATE_SPACE_STEP === "absent") {
  if (record) {
    throw new Error(
      `deleted private target still exists: ${JSON.stringify(record)}`,
    );
  }
} else {
  throw new Error(`unknown private-space step ${PRIVATE_SPACE_STEP}`);
}
