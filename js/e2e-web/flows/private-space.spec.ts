import {
  expect,
  test,
  type APIRequestContext,
  type Page,
} from "@playwright/test";
import { randomUUID } from "node:crypto";
import { loginThroughPds } from "./login";

const HTTPS_URL = process.env.SERVER_HTTPS_URL;
const DID = process.env.ACCOUNT_DID!;
const PDS_KIND = process.env.PDS_KIND ?? "reference";
const COLLECTION = "place.stream.multistream.target";
const SPACE = `at://${DID}/space/place.stream.personal/self`;
const LEGACY_NAME = "e2e legacy target";
const LEGACY_URL = "rtmp://127.0.0.1:9/live/e2e-legacy";

test.skip(
  !HTTPS_URL || !process.env.PDS_HTTPS_URL,
  "harness started without its HTTPS/OAuth hostnames",
);
test.skip(
  !!process.env.E2E_UPSTREAM_STATION,
  "private-space fixtures belong to the standalone harness",
);

interface Target {
  uri: string;
  record: { name?: string; url: string; active: boolean };
}

interface SpaceRecord {
  rkey?: string;
  uri?: string;
  value: { name?: string; url: string; active: boolean };
}

async function nodeTargets(page: Page, action: () => Promise<unknown>) {
  const [response] = await Promise.all([
    page.waitForResponse(
      (res) =>
        new URL(res.url()).pathname ===
          "/xrpc/place.stream.multistream.listTargets" &&
        res.request().method() === "GET",
    ),
    action(),
  ]);
  expect(response.ok(), await response.text()).toBe(true);
  const { targets } = (await response.json()) as { targets: Target[] };
  return targets;
}

async function mutateTarget(
  page: Page,
  method: "createTarget" | "putTarget" | "deleteTarget",
  action: () => Promise<unknown>,
) {
  const [response] = await Promise.all([
    page.waitForResponse(
      (res) =>
        new URL(res.url()).pathname ===
          `/xrpc/place.stream.multistream.${method}` &&
        res.request().method() === "POST",
    ),
    action(),
  ]);
  expect(response.ok(), await response.text()).toBe(true);
  if (method !== "deleteTarget") {
    expect(response.request().postDataJSON().multistreamTarget.active).toBe(
      false,
    );
  }
}

async function spaceRecords(request: APIRequestContext) {
  const method =
    PDS_KIND === "vlpds"
      ? "vlpds.admin.listSpaceRecords"
      : "com.atproto.space.listRecords";
  // The reference fixture's password is js/dev-env/lib/constants.ts ADMIN_PASSWORD.
  const password = PDS_KIND === "vlpds" ? "dev-admin-token" : "admin-pass";
  const records: SpaceRecord[] = [];
  let cursor: string | undefined;
  do {
    const response = await request.get(
      `${process.env.PDS_HTTP_URL}/xrpc/${method}`,
      {
        headers: {
          Authorization: `Basic ${Buffer.from(`admin:${password}`).toString("base64")}`,
        },
        params: {
          space: SPACE,
          repo: DID,
          collection: COLLECTION,
          limit: 100,
          ...(cursor ? { cursor } : {}),
        },
      },
    );
    expect(response.ok(), await response.text()).toBe(true);
    const body = (await response.json()) as {
      records: SpaceRecord[];
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

function expectTargetURI(target: Target) {
  const rkey = target.uri.split("/").pop()!;
  expect(rkey).not.toBe("");
  expect(target.uri).toBe(`at://${DID}/${COLLECTION}/${rkey}`);
  return rkey;
}

async function expectSpaceTarget(
  request: APIRequestContext,
  target: Target,
  name: string,
  url: string,
) {
  const rkey = expectTargetURI(target);
  if (PDS_KIND === "reference") return;
  await expect
    .poll(async () =>
      (await spaceRecords(request)).find((record) => record.rkey === rkey),
    )
    .toMatchObject({ rkey, value: { name, url, active: false } });
}

for (const frontend of ["app", "web"]) {
  test(`private-space: ${frontend} migrates legacy targets and persists inactive UI CRUD`, async ({
    page,
    request,
  }) => {
    expect(["reference", "spaces", "vlpds"]).toContain(PDS_KIND);
    await loginThroughPds(page);
    if (frontend === "web") {
      await page
        .context()
        .addCookies([{ name: "sp_web_beta", value: "1", url: HTTPS_URL! }]);
    }
    const screen = `${HTTPS_URL}${
      frontend === "app"
        ? "/settings/streaming/multistream"
        : "/dashboard/multistream/"
    }`;
    const name = `Private space ${frontend} ${randomUUID()}`;
    const renamed = `${name} renamed`;
    const url = `rtmp://127.0.0.1:9/live/${randomUUID()}`;
    const refresh = () =>
      nodeTargets(page, () =>
        page.getByTestId("multistream-target-refresh").click(),
      );
    // Rkeys identify rows even when target names are blank, repeated, or edited.
    const row = (target: Target) =>
      page.getByTestId(`multistream-target-row-${target.uri.split("/").pop()}`);
    const remove = async (target: Target) => {
      await row(target)
        .getByTestId(/^multistream-target-delete-/)
        .click();
      await mutateTarget(page, "deleteTarget", () =>
        page.getByTestId("multistream-target-confirm-delete").click(),
      );
      await expect(row(target)).toBeHidden();
    };
    let creationAttempted = false;
    try {
      const initial = await nodeTargets(page, () => page.goto(screen));
      await expect(page.getByText(LEGACY_NAME, { exact: true })).toBeVisible();
      const legacy = initial.find(
        (target) => target.record.name === LEGACY_NAME,
      );
      expect(legacy).toBeDefined();
      expect(legacy!.record).toMatchObject({ url: LEGACY_URL, active: false });
      for (const target of initial) expectTargetURI(target);
      await expectSpaceTarget(request, legacy!, LEGACY_NAME, LEGACY_URL);

      await page.getByTestId("multistream-target-create").click();
      await page
        .getByLabel("Multistream target name", { exact: true })
        .fill(name);
      await page
        .getByLabel("Multistream target URL", { exact: true })
        .fill(url);
      const active = page.getByRole("switch", {
        name: "Multistream target active",
        exact: true,
      });
      await expect(active).toBeChecked();
      await active.click();
      await expect(active).not.toBeChecked();
      creationAttempted = true;
      await mutateTarget(page, "createTarget", () =>
        page.getByTestId("multistream-target-save").click(),
      );
      await expect(page.getByText(name, { exact: true })).toBeVisible();
      const created = (await refresh()).find(
        (target) => target.record.name === name,
      );
      expect(created).toBeDefined();
      expect(created!.record).toMatchObject({ name, url, active: false });
      await expectSpaceTarget(request, created!, name, url);

      await row(created!)
        .getByTestId(/^multistream-target-edit-/)
        .click();
      await page
        .getByLabel("Multistream target name", { exact: true })
        .fill(renamed);
      await expect(active).not.toBeChecked();
      await mutateTarget(page, "putTarget", () =>
        page.getByTestId("multistream-target-save").click(),
      );
      await expect(page.getByText(renamed, { exact: true })).toBeVisible();
      await expect(page.getByText(name, { exact: true })).toBeHidden();
      const updated = (await refresh()).find(
        (target) => target.uri === created!.uri,
      );
      expect(updated).toBeDefined();
      expect(updated!.record).toMatchObject({
        name: renamed,
        url,
        active: false,
      });
      await expectSpaceTarget(request, updated!, renamed, url);

      await remove(updated!);
      expect(
        (await refresh()).some((target) => target.uri === created!.uri),
      ).toBe(false);
      if (PDS_KIND !== "reference") {
        const rkey = expectTargetURI(created!);
        await expect
          .poll(async () =>
            (await spaceRecords(request)).some(
              (record) => record.rkey === rkey,
            ),
          )
          .toBe(false);
      }
    } finally {
      // Reload also dismisses an unfinished form before cleaning up a failed run.
      if (creationAttempted) {
        const remaining = await nodeTargets(page, () => page.goto(screen));
        for (const target of remaining) {
          if (target.record.name === name || target.record.name === renamed) {
            await remove(target);
          }
        }
      }
    }
  });
}
