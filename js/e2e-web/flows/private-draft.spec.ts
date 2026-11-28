import { expect, test, type Page } from "@playwright/test";
import { randomUUID } from "node:crypto";
import { DatabaseSync } from "node:sqlite";
import { loginThroughPds } from "./login";
import {
  expectDraftMetadata,
  supportsSpaceAdmin,
  withServiceAuth,
} from "./private-space-fixture";

const HTTPS_URL = process.env.SERVER_HTTPS_URL;
const PDS_HTTPS_URL = process.env.PDS_HTTPS_URL;
const PDS_HTTP_URL = process.env.PDS_HTTP_URL;

test.skip(
  !HTTPS_URL || !PDS_HTTPS_URL || !PDS_HTTP_URL,
  "requires the HTTPS/OAuth harness",
);
test.skip(
  !!process.env.E2E_UPSTREAM_STATION,
  "requires the standalone harness",
);

type Draft = {
  uri: string;
  record: {
    title: string;
    description?: string;
    status: string;
    createdAt: string;
  };
};

function expectProcessingDraft(
  draft: Draft,
  uri: string,
  metadata: { title: string; description: string },
  createdAt: string,
) {
  expect(draft.uri === uri, "draft URI is unchanged").toBe(true);
  expect(
    draft.record.title === metadata.title &&
      draft.record.description === metadata.description,
    "draft has the expected editable metadata",
  ).toBe(true);
  expect(draft.record.status).toBe("processing");
  expect(draft.record.createdAt).toBe(createdAt);
}

function draftResponse(page: Page, method: string) {
  return page.waitForResponse(
    (res) => new URL(res.url()).pathname === `/xrpc/place.stream.vod.${method}`,
  );
}

function sqliteDraftData(uri: string): Buffer {
  const db = new DatabaseSync(process.env.E2E_DRAFT_STATE_DB!, {
    readOnly: true,
    timeout: 5_000,
  });
  try {
    const row = db
      .prepare("SELECT data FROM draft_videos WHERE uri = ?")
      .get(uri);
    if (!(row?.data instanceof Uint8Array)) {
      throw new Error("draft is missing its operational SQLite row");
    }
    return Buffer.from(row.data);
  } finally {
    db.close();
  }
}

test("private drafts: OAuth migrates legacy metadata and keeps edits private", async ({
  page,
  request,
}) => {
  // A separate account gives this test (and retries) exactly one draft and no
  // prior grant, even when another test already logged in and uploaded media.
  const label = `draft-${randomUUID().slice(0, 8)}`;
  const handle = `${label}.${new URL(PDS_HTTPS_URL!).hostname}`;
  const password = "e2e draft password";
  const account = await request.post(
    `${PDS_HTTP_URL}/xrpc/com.atproto.server.createAccount`,
    { data: { handle, email: `${label}@example.com`, password } },
  );
  expect(account.ok(), `createAccount: HTTP ${account.status()}`).toBe(true);
  const { did, accessJwt } = (await account.json()) as {
    did: string;
    accessJwt: string;
  };
  const service = (
    method: string,
    options?: Parameters<typeof withServiceAuth>[3],
  ) =>
    withServiceAuth(request, accessJwt, `place.stream.vod.${method}`, options);

  const created = await service("createDraft", { data: {} });
  expect(created.ok(), `createDraft: HTTP ${created.status()}`).toBe(true);
  const { uri } = (await created.json()) as { uri: string };
  const rkey = uri.split("/").pop()!;
  expect(rkey).toMatch(/^[234567a-z]{13}$/);
  expect(
    uri ===
      `ats://${did}/place.stream.vod.drafts/self/${did}/place.stream.vod.draftVideo/${rkey}`,
    "draft has the canonical URI",
  ).toBe(true);
  const legacy = {
    title: "Legacy draft metadata",
    description: "Before OAuth",
  };
  expect(
    (await service("updateDraft", { data: { uri, ...legacy } })).ok(),
  ).toBe(true);
  const listed = await service("listDrafts");
  expect(listed.ok()).toBe(true);
  const { drafts: before } = (await listed.json()) as { drafts: Draft[] };
  expect(before.length).toBe(1);
  const { createdAt } = before[0].record;
  expect(Number.isFinite(Date.parse(createdAt))).toBe(true);
  expectProcessingDraft(before[0], uri, legacy, createdAt);
  const legacyData = sqliteDraftData(uri);
  expect(legacyData.includes(legacy.title)).toBe(true);
  expect(legacyData.includes(legacy.description)).toBe(true);

  await loginThroughPds(page, HTTPS_URL, { handle, password });
  const listURL = `${HTTPS_URL}/upload/drafts`;
  const editorURL = `${HTTPS_URL}/upload/video/${rkey}`;
  const [initial] = await Promise.all([
    draftResponse(page, "listDrafts"),
    page.goto(listURL),
  ]);
  expect(initial.ok()).toBe(true);
  const { drafts: migrated } = (await initial.json()) as { drafts: Draft[] };
  expect(migrated.length).toBe(1);
  expectProcessingDraft(migrated[0], uri, legacy, createdAt);
  await expect(page.getByText(legacy.title, { exact: true })).toBeVisible();
  await expectDraftMetadata(request, did, rkey, legacy);
  if (supportsSpaceAdmin) {
    const operational = sqliteDraftData(uri);
    expect(operational.includes(legacy.title)).toBe(false);
    expect(operational.includes(legacy.description)).toBe(false);
  }

  const expectReauthorization = async (
    method: string,
    options?: Parameters<typeof service>[1],
  ) => {
    const response = await service(method, options);
    expect(response.status(), method).toBe(403);
    const { error, message } = (await response.json()) as {
      error?: string;
      message?: string;
    };
    expect(
      /re.?authoriz/i.test(`${error} ${message}`),
      `${method} requires reauthorization`,
    ).toBe(true);
  };
  if (supportsSpaceAdmin) {
    await expectReauthorization("getDraft", { params: { uri } });
    await expectReauthorization("updateDraft", {
      data: { uri, title: "No grant" },
    });
    await expectReauthorization("deleteDraft", { data: { uri } });
  } else {
    const fallback = await service("getDraft", { params: { uri } });
    expect(fallback.ok()).toBe(true);
    const { draft } = (await fallback.json()) as { draft: Draft };
    expectProcessingDraft(draft, uri, legacy, createdAt);
  }

  await page.goto(editorURL);
  const title = page.getByTestId("vod-editor-title");
  const description = page.getByPlaceholder("Describe your video…");
  await expect(title).toHaveValue(legacy.title);
  const edited = { title: "Updated private draft", description: "After OAuth" };
  await title.fill(edited.title);
  await description.fill(edited.description);
  const [saved] = await Promise.all([
    draftResponse(page, "updateDraft"),
    page.getByRole("button", { name: "Save Draft", exact: true }).click(),
  ]);
  expect(saved.ok()).toBe(true);
  const { draft: updated } = (await saved.json()) as { draft: Draft };
  expectProcessingDraft(updated, uri, edited, createdAt);
  await page.reload();
  await expect(title).toHaveValue(edited.title);
  await expect(description).toHaveValue(edited.description);
  await expect(
    page.getByText("Editing draft · Processing", { exact: true }),
  ).toBeVisible();
  await expectDraftMetadata(request, did, rkey, edited);
  // CBOR stores these UTF-8 strings verbatim, as verified before migration.
  const operational = sqliteDraftData(uri);
  expect(operational.includes(edited.title)).toBe(!supportsSpaceAdmin);
  expect(operational.includes(edited.description)).toBe(!supportsSpaceAdmin);
  // Neither the client-facing draft type nor its private metadata is a public
  // repo record. Publishing (covered by the upload fixture) is the public write.
  for (const collection of ["draftVideo", "draftMetadata"]) {
    const response = await request.get(
      `${PDS_HTTP_URL}/xrpc/com.atproto.repo.listRecords`,
      { params: { repo: did, collection: `place.stream.vod.${collection}` } },
    );
    expect(response.ok()).toBe(true);
    expect((await response.json()).records.length).toBe(0);
  }

  const [deleted] = await Promise.all([
    draftResponse(page, "deleteDraft"),
    page.getByRole("button", { name: "Delete draft", exact: true }).click(),
  ]);
  expect(deleted.ok()).toBe(true);
  await page.waitForURL((url) => /^\/upload\/videos\/?$/.test(url.pathname));
  const [remaining] = await Promise.all([
    draftResponse(page, "listDrafts"),
    page.goto(listURL),
  ]);
  expect(remaining.ok()).toBe(true);
  expect((await remaining.json()).drafts.length).toBe(0);
  await expect(page.getByText("No drafts yet", { exact: true })).toBeVisible();
  await expectDraftMetadata(request, did, rkey, null);
  if (supportsSpaceAdmin) {
    // The account's storage mode must not depend on a draft still existing.
    await expectReauthorization("listDrafts");
    await expectReauthorization("createDraft", { data: {} });
  } else {
    const remaining = await service("listDrafts");
    expect(remaining.ok()).toBe(true);
    expect((await remaining.json()).drafts.length).toBe(0);
  }
});
