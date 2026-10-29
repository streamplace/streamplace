// The chat record chat-timezone.yaml sent, as the account's PDS stores it.
const res = http.get(
  `${PDS_HTTP_URL}/xrpc/com.atproto.repo.listRecords?repo=${encodeURIComponent(ACCOUNT_DID)}&collection=place.stream.chat.message&limit=100`,
);
if (!res.ok) {
  throw new Error(`listRecords: HTTP ${res.status}: ${res.body}`);
}
const text = `timezone check ${output.timezoneSentAfter}`;
const record = json(res.body).records.find((r) => r.value.text === text);
if (!record) {
  throw new Error(`no place.stream.chat.message record says "${text}"`);
}
const { createdAt } = record.value;
console.log(`createdAt ${createdAt}, device offset ${DEVICE_UTC_OFFSET}`);

// RFC 3339 with the device's numeric offset. Z, as toISOString() writes,
// would drop the sender's local wall-clock time.
const offset =
  /^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(?:\.\d+)?([+-]\d{2}:\d{2})$/.exec(
    createdAt,
  )?.[1];
if (offset !== DEVICE_UTC_OFFSET) {
  throw new Error(
    `createdAt ${createdAt} should end in the device's offset ${DEVICE_UTC_OFFSET}`,
  );
}

// And the instant it was sent. The device's clock may drift from this host's
// a little; a wrong offset for any zone but UTC is off by 30 minutes or more.
const skew = 2 * 60 * 1000;
const sent = Date.parse(createdAt);
if (!(sent >= output.timezoneSentAfter - skew && sent <= Date.now() + skew)) {
  throw new Error(
    `createdAt ${createdAt} is not when the message was sent, after ${new Date(output.timezoneSentAfter).toISOString()}`,
  );
}
