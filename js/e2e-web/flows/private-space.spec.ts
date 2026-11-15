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
  latestEvent?: { status: string };
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
  active = false,
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
      active,
    );
  }
}

// A service-auth caller has no space grant, so the node answers it from statedb.
async function statedbTargets(request: APIRequestContext) {
  const pds = `${process.env.PDS_HTTP_URL}/xrpc`;
  const lxm = "place.stream.multistream.listTargets";
  const session = await request.post(
    `${pds}/com.atproto.server.createSession`,
    {
      data: { identifier: DID, password: process.env.ACCOUNT_PASSWORD },
    },
  );
  expect(session.ok(), await session.text()).toBe(true);
  const auth = await request.get(`${pds}/com.atproto.server.getServiceAuth`, {
    headers: { Authorization: `Bearer ${(await session.json()).accessJwt}` },
    params: { aud: `did:web:${new URL(HTTPS_URL!).host}`, lxm },
  });
  expect(auth.ok(), await auth.text()).toBe(true);
  const response = await request.get(`${process.env.SERVER_URL}/xrpc/${lxm}`, {
    headers: { Authorization: `Bearer ${(await auth.json()).token}` },
  });
  expect(response.ok(), await response.text()).toBe(true);
  return ((await response.json()) as { targets: Target[] }).targets;
}

// Counts the space change notifications the node has accepted from the PDS.
async function spaceNotifications(request: APIRequestContext) {
  const response = await request.get(
    `${process.env.SERVER_INTERNAL_URL}/metrics`,
  );
  const count = (await response.text()).match(
    /^streamplace_space_notifications_total (\d+)/m,
  );
  expect(count, "the node's metrics list the counter").not.toBeNull();
  return Number(count![1]);
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
  test(`private-space: ${frontend} moves legacy targets to the space and keeps UI CRUD there`, async ({
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
      // Once moved, the space is the only store.
      expect(
        (await statedbTargets(request)).map((target) => target.record.name),
      ).toEqual(PDS_KIND === "reference" ? [LEGACY_NAME] : []);

      await page.getByTestId("multistream-target-create").click();
      await page.getByTestId("multistream-target-form-name").fill(name);
      await page.getByTestId("multistream-target-form-url").fill(url);
      const active = page
        .getByTestId("multistream-target-form-active")
        .getByRole("switch");
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

      // The director drives an active target from the store that holds it.
      await row(created!)
        .getByTestId(/^multistream-target-edit-/)
        .click();
      await expect(active).not.toBeChecked();
      await active.click();
      await mutateTarget(
        page,
        "putTarget",
        () => page.getByTestId("multistream-target-save").click(),
        true,
      );
      await expect
        .poll(
          async () =>
            (await refresh()).find((target) => target.uri === created!.uri)
              ?.latestEvent?.status,
          { timeout: 60_000 },
        )
        .toBeTruthy();

      // By now the director follows the space, so the PDS tells it of a write.
      const notified = await spaceNotifications(request);
      await row(created!)
        .getByTestId(/^multistream-target-edit-/)
        .click();
      await page.getByTestId("multistream-target-form-name").fill(renamed);
      await expect(active).toBeChecked();
      await active.click();
      await mutateTarget(page, "putTarget", () =>
        page.getByTestId("multistream-target-save").click(),
      );
      if (PDS_KIND !== "reference") {
        await expect
          .poll(() => spaceNotifications(request))
          .toBeGreaterThan(notified);
      }
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
      creationAttempted = false;
    } finally {
      // Only a failed run leaves a target. Leaving the page first dismisses an
      // unfinished form and keeps its late responses out of the reload's.
      if (creationAttempted) {
        await page.goto("about:blank");
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
