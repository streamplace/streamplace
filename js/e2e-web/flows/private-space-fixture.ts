import { expect, type APIRequestContext } from "@playwright/test";

const PDS_KIND = process.env.PDS_KIND ?? "reference";
const pds = `${process.env.PDS_HTTP_URL}/xrpc`;
export const supportsSpaceAdmin = PDS_KIND === "spaces" || PDS_KIND === "vlpds";

interface SpaceRecord<T> {
  rkey?: string;
  uri?: string;
  value: T;
}

export async function spaceRecords<T>(
  request: APIRequestContext,
  repo: string,
  collection: string,
) {
  const method =
    PDS_KIND === "vlpds"
      ? "vlpds.admin.listSpaceRecords"
      : "com.atproto.space.listRecords";
  const password = PDS_KIND === "vlpds" ? "dev-admin-token" : "admin-pass";
  const records: SpaceRecord<T>[] = [];
  let cursor: string | undefined;
  do {
    const response = await request.get(`${pds}/${method}`, {
      headers: {
        Authorization: `Basic ${Buffer.from(`admin:${password}`).toString("base64")}`,
      },
      params: {
        space: `at:space/${repo}/place.stream.personal/self`,
        repo,
        collection,
        limit: 100,
        ...(cursor ? { cursor } : {}),
      },
    });
    expect(response.ok(), `${method}: HTTP ${response.status()}`).toBe(true);
    const body = (await response.json()) as {
      records: SpaceRecord<T>[];
      cursor?: string;
    };
    records.push(...body.records);
    cursor = body.cursor;
  } while (cursor);
  return records.map((record) => ({
    ...record,
    rkey: record.rkey ?? record.uri?.split("/").pop(),
  }));
}

export async function withServiceAuth(
  request: APIRequestContext,
  accessJwt: string,
  lxm: string,
  options: {
    params?: Record<string, string>;
    data?: Record<string, unknown>;
  } = {},
) {
  const auth = await request.get(`${pds}/com.atproto.server.getServiceAuth`, {
    headers: { Authorization: `Bearer ${accessJwt}` },
    params: {
      aud: `did:web:${new URL(process.env.SERVER_HTTPS_URL!).host}`,
      lxm,
    },
  });
  expect(auth.ok(), `getServiceAuth: HTTP ${auth.status()}`).toBe(true);
  return request.fetch(`${process.env.SERVER_URL}/xrpc/${lxm}`, {
    method: options.data ? "POST" : "GET",
    headers: { Authorization: `Bearer ${(await auth.json()).token}` },
    ...options,
  });
}

const draftMetadataCollection = "place.stream.vod.draftMetadata";
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

export async function expectDraftMetadata(
  request: APIRequestContext,
  did: string,
  rkey: string,
  expected: { title: string; description?: string } | null,
) {
  if (!supportsSpaceAdmin) return;
  await expect
    .poll(
      async () => {
        const records = await spaceRecords<Record<string, unknown>>(
          request,
          did,
          draftMetadataCollection,
        );
        const record = records.find((entry) => entry.rkey === rkey);
        if (!expected) return !record;
        return (
          !!record &&
          record.value.title === expected.title &&
          (expected.description === undefined ||
            record.value.description === expected.description) &&
          Object.keys(record.value).every((field) =>
            metadataFields.includes(field),
          )
        );
      },
      {
        message: expected
          ? "space contains only the expected draft metadata"
          : "draft metadata removed from space",
      },
    )
    .toBe(true);
}
