// Maestro runs on the host: read the PDS's loopback admin API, not device HTTPS.
if (PDS_KIND !== "spaces" && PDS_KIND !== "vlpds") {
  throw new Error(`private-space admin read is not supported for ${PDS_KIND}`);
}
const isDraft = typeof PRIVATE_DRAFT_STEP !== "undefined";
const collection = isDraft
  ? "place.stream.vod.draftMetadata"
  : "place.stream.multistream.target";
const space = `at:space/${ACCOUNT_DID}/place.stream.personal/self`;
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
    `${PDS_HTTP_URL}/xrpc/${method}?space=${encodeURIComponent(space)}&repo=${encodeURIComponent(ACCOUNT_DID)}&collection=${collection}&limit=100${cursor ? `&cursor=${encodeURIComponent(cursor)}` : ""}`,
    { headers: { Authorization: authorization } },
  );
  if (!res.ok) {
    throw new Error(`${method}: HTTP ${res.status}`);
  }
  const body = json(res.body);
  records.push(...body.records);
  cursor = body.cursor;
} while (cursor);
const rkey = (record) => record.rkey || record.uri?.split("/").pop();
if (isDraft) {
  const record = records.find((r) => rkey(r) === output.privateDraftRkey);
  if (PRIVATE_DRAFT_STEP === "deleted") {
    if (record) throw new Error("deleted draft metadata still in the space");
  } else {
    const title =
      PRIVATE_DRAFT_STEP === "migrated" ? "Untitled" : output.privateDraftTitle;
    if (!record || record.value.title !== title) {
      throw new Error("missing draft metadata for the original draft key");
    }
    const metadataFields = [
      "$type",
      "title",
      "description",
      "descriptionFacets",
      "thumb",
      "activity",
      "tags",
      "contentRights",
      "contentWarnings",
    ];
    if (
      Object.keys(record.value).some((key) => !metadataFields.includes(key))
    ) {
      throw new Error("draft space record contains non-metadata fields");
    }
  }
} else {
  const record = records.find(
    (r) =>
      r.value.name === output.privateSpaceName ||
      (output.privateSpaceRkey && rkey(r) === output.privateSpaceRkey),
  );
  if (PRIVATE_SPACE_STEP === "present") {
    if (!record) throw new Error("private target missing");
    if (record.value.url !== output.privateSpaceUrl) {
      throw new Error("private target URL does not match");
    }
    output.privateSpaceRkey = rkey(record);
    if (!output.privateSpaceRkey) throw new Error("private target has no rkey");
  } else if (PRIVATE_SPACE_STEP === "absent") {
    if (record) throw new Error("deleted private target still exists");
  } else {
    throw new Error(`unknown private-space step ${PRIVATE_SPACE_STEP}`);
  }
}
