import { expect, test, type Page } from "@playwright/test";
import { execFileSync } from "node:child_process";
import { createHash } from "node:crypto";
import { existsSync, readFileSync, writeFileSync } from "node:fs";
import { join } from "node:path";
import { stripVTControlCharacters } from "node:util";
import { loginThroughPds } from "./login";

// Semantic native counterpart: .maestro/logged-in/upstream-station.yaml.
// All three servers and the MoQ bridge are real harness processes. SQLite
// observation is read-only except a renewal timestamp fixture; genuine keys,
// grants and responses are unchanged. No production endpoints or mocks.
test.skip(!process.env.E2E_UPSTREAM_STATION, "requires upstream harness mode");

const LOCAL = process.env.SERVER_URL!;
const STATION = process.env.UPSTREAM_STATION_URL!;
const DID = process.env.ACCOUNT_DID!;
const UNRELATED = process.env.UNRELATED_ACCOUNT_DID!;

type Session = {
  downstream_dpop_jkt: string;
  upstream_dpop_private_jwk: string;
  upstream_access_token: string;
  downstream_refresh_token: string;
  upstream_auth_server_issuer: string;
  pds_url: string;
  downstream_revoked_at: string | null;
};

function query<T>(
  node: "local" | "station",
  database: "index" | "state",
  sql: string,
  params: unknown[] = [],
): T[] {
  const dir =
    process.env[
      node === "local" ? "E2E_LOCAL_DATA_DIR" : "E2E_STATION_DATA_DIR"
    ]!;
  return JSON.parse(
    execFileSync(
      "python3",
      [
        "-c",
        `
import glob, json, sqlite3, sys
root, kind, sql, params = sys.argv[1:]
paths = glob.glob(root + '/index/index_*.sqlite') if kind == 'index' else [root + '/state.sqlite']
if len(paths) != 1:
    raise RuntimeError('expected exactly one harness database: ' + repr(paths))
conn = sqlite3.connect('file:' + paths[0] + '?mode=ro', uri=True, timeout=10)
conn.row_factory = sqlite3.Row
print(json.dumps([dict(row) for row in conn.execute(sql, json.loads(params))]))
`,
        dir,
        database,
        sql,
        JSON.stringify(params),
      ],
      { encoding: "utf8" },
    ),
  );
}

function sessions(node: "local" | "station", jkt: string): Session[] {
  return query<Session>(
    node,
    "state",
    `SELECT downstream_dpop_jkt, upstream_dpop_private_jwk,
    upstream_access_token, downstream_refresh_token, upstream_auth_server_issuer, pds_url, downstream_revoked_at
    FROM oauth_sessions WHERE repo_did = ? AND revoked_at IS NULL AND downstream_dpop_jkt = ?`,
    [DID, jkt],
  );
}

function thumbprint(jwk: {
  kty: string;
  crv: string;
  x: string;
  y: string;
}): string {
  return createHash("sha256")
    .update(JSON.stringify({ crv: jwk.crv, kty: jwk.kty, x: jwk.x, y: jwk.y }))
    .digest("base64url");
}

type PdsRequest = { method: string; path: string; query: string };
function pdsAudit(): PdsRequest[] {
  return readFileSync(process.env.E2E_LOCAL_PDS_AUDIT!, "utf8")
    .split("\n")
    .filter(Boolean)
    .map((line) => JSON.parse(line));
}
function repoReads(): number {
  return pdsAudit().filter((entry) =>
    /\/(com\.atproto\.sync\.(getRepo|getLatestCommit)|com\.atproto\.repo\.listRecords)$/.test(
      entry.path,
    ),
  ).length;
}

async function sendChat(
  page: Page,
  message: string,
): Promise<{ uri: string; jkt: string }> {
  const created = page.waitForResponse(
    (response) =>
      response.request().method() === "POST" &&
      new URL(response.url()).origin === LOCAL &&
      new URL(response.url()).pathname ===
        "/xrpc/com.atproto.repo.createRecord" &&
      response.request().postData()?.includes(message) === true,
  );
  const input = page.getByPlaceholder("Type a message...");
  await input.fill(message);
  await expect(
    page.getByRole("button", { name: "Send", exact: true }),
  ).toBeEnabled();
  await input.press("Enter");
  const response = await created;
  expect(
    response.ok(),
    `authenticated local write returned ${response.status()}`,
  ).toBe(true);
  const dpop = (await response.request().allHeaders())["dpop"];
  expect(dpop).toBeTruthy();
  const header = JSON.parse(
    Buffer.from(dpop.split(".")[0], "base64url").toString(),
  );
  const payload = JSON.parse(
    Buffer.from(dpop.split(".")[1], "base64url").toString(),
  );
  expect(payload.htu).toBe(`${LOCAL}/xrpc/com.atproto.repo.createRecord`);
  const { uri } = await response.json();
  expect(uri).toMatch(
    new RegExp(`^at://${DID}/place\\.stream\\.chat\\.message/`),
  );
  // Do not accept the optimistic bubble: the exact PDS-created URI must arrive
  // in the local durable index, after the initial OAuth repo sync.
  await expect
    .poll(
      () =>
        query<{ uri: string }>(
          "local",
          "index",
          "SELECT uri FROM chat_messages WHERE uri = ?",
          [uri],
        ).length,
    )
    .toBe(1);
  return { uri, jkt: thumbprint(header.jwk) };
}

async function refreshAtLocal(page: Page): Promise<void> {
  // Expire only the SDK's persisted timestamp, then let the real client renew
  // and persist its rotated tokens. Hand-signing a refresh behind the SDK would
  // leave its logout holding a stale refresh token.
  await page.evaluate(async (did) => {
    const db = await new Promise<IDBDatabase>((resolve, reject) => {
      const opened = indexedDB.open("@atproto-oauth-client");
      opened.onsuccess = () => resolve(opened.result);
      opened.onerror = () => reject(opened.error);
    });
    try {
      await new Promise<void>((resolve, reject) => {
        const transaction = db.transaction("session", "readwrite");
        transaction.oncomplete = () => resolve();
        transaction.onerror = () => reject(transaction.error);
        transaction.onabort = () => reject(transaction.error);
        const cursor = transaction.objectStore("session").openCursor();
        cursor.onsuccess = () => {
          const row = cursor.result;
          if (!row) return;
          const item = row.value;
          if (item.value.tokenSet.sub === did) {
            item.value.tokenSet.expires_at = new Date(0).toISOString();
            row.update(item);
          }
          row.continue();
        };
      });
    } finally {
      db.close();
    }
  }, DID);
  const refreshed = page.waitForResponse(
    (response) =>
      response.url() === `${LOCAL}/oauth/token` &&
      response.request().postData()?.includes("grant_type=refresh_token") ===
        true &&
      response.ok(),
  );
  const restoredProfile = page.waitForResponse(
    (response) =>
      new URL(response.url()).origin === LOCAL &&
      new URL(response.url()).pathname === "/xrpc/app.bsky.actor.getProfile" &&
      response.ok(),
  );
  await page.reload();
  const body = await (await refreshed).json();
  expect(body.sub).toBe(DID);
  expect(body.token_type).toBe("DPoP");
  expect(body.access_token).toBeTruthy();
  expect((await (await restoredProfile).json()).did).toBe(DID);
}

test("upstream-station: local login, isolated DPoP hops, MoQ indexing, refresh and revoke", async ({
  page,
  request,
}) => {
  test.setTimeout(300_000);
  await loginThroughPds(page);
  await page.goto(`${LOCAL}/`);
  await page.getByTestId("home-stream-card").first().click();
  await expect(
    page.getByText("Now streaming - e2e test stream").first(),
  ).toBeVisible();
  await expect
    .poll(
      () =>
        query<{ pds: string; version: string }>(
          "local",
          "index",
          "SELECT pds, version FROM repos WHERE did = ?",
          [DID],
        )[0]?.version,
    )
    .toBeTruthy();
  const readsBefore = repoReads();
  const message = `upstream MoQ ${Date.now()}`;
  const { uri, jkt } = await sendChat(page, message);
  expect(
    repoReads(),
    "live MoQ commit must not fall back to a PDS repo scrape",
  ).toBe(readsBefore);
  expect(
    query<{ pds: string }>(
      "local",
      "index",
      "SELECT pds FROM repos WHERE did = ?",
      [DID],
    )[0].pds,
  ).toBe(process.env.PDS_HTTPS_URL);

  const localSession = sessions("local", jkt)[0];
  expect(localSession).toBeDefined();
  expect(localSession.pds_url).toBe(STATION);
  expect(localSession.upstream_auth_server_issuer).toBe(STATION);
  const stationJkt = thumbprint(
    JSON.parse(localSession.upstream_dpop_private_jwk),
  );
  expect(stationJkt).not.toBe(jkt);
  const stationSession = sessions("station", stationJkt)[0];
  expect(stationSession).toBeDefined();
  expect(stationSession.pds_url).toBe(process.env.PDS_HTTPS_URL);
  const pdsJkt = thumbprint(
    JSON.parse(stationSession.upstream_dpop_private_jwk),
  );
  expect(pdsJkt).not.toBe(stationJkt);
  expect(pdsJkt).not.toBe(jkt);
  const localTokenBefore = createHash("sha256")
    .update(localSession.upstream_access_token)
    .digest("hex");
  const stationTokenBefore = createHash("sha256")
    .update(stationSession.upstream_access_token)
    .digest("hex");
  // Deterministic renewal timing only: the proxies still perform genuine,
  // separately DPoP-bound refresh exchanges with the station and the PDS.
  for (const [dir, key] of [
    [process.env.E2E_LOCAL_DATA_DIR!, jkt],
    [process.env.E2E_STATION_DATA_DIR!, stationJkt],
  ]) {
    execFileSync("python3", [
      "-c",
      `
import datetime, sqlite3, sys
conn = sqlite3.connect(sys.argv[1] + '/state.sqlite', timeout=10)
expired = (datetime.datetime.now(datetime.timezone.utc) - datetime.timedelta(minutes=1)).isoformat()
conn.execute('UPDATE oauth_sessions SET upstream_access_token_exp = ? WHERE downstream_dpop_jkt = ?', (expired, sys.argv[2]))
conn.commit()
`,
      dir,
      key,
    ]);
  }
  await refreshAtLocal(page);
  await sendChat(page, `write after real chained refresh ${Date.now()}`);
  await expect
    .poll(() =>
      createHash("sha256")
        .update(sessions("local", jkt)[0].upstream_access_token)
        .digest("hex"),
    )
    .not.toBe(localTokenBefore);
  await expect
    .poll(() =>
      createHash("sha256")
        .update(sessions("station", stationJkt)[0].upstream_access_token)
        .digest("hex"),
    )
    .not.toBe(stationTokenBefore);

  // Publish an unrelated commit AFTER local OAuth, through its real PDS login.
  const unrelatedResponse = await request.post(
    `${process.env.E2E_PDS_INTERNAL_URL}/xrpc/com.atproto.repo.createRecord`,
    {
      headers: { Authorization: `Bearer ${process.env.UNRELATED_ACCESS_JWT}` },
      data: {
        repo: UNRELATED,
        collection: "place.stream.livestream",
        record: {
          $type: "place.stream.livestream",
          createdAt: new Date().toISOString(),
          title: `unrelated after login ${Date.now()}`,
        },
      },
    },
  );
  expect(unrelatedResponse.ok()).toBe(true);
  // A subsequent selected-user event gives the real bridge time to process
  // both commits, without using a sleep as the indexing assertion.
  await sendChat(page, `selected user after unrelated ${Date.now()}`);
  assertScopedIndex();

  await page.reload();
  await expect(page.getByText(message).first()).toBeVisible();
  expect(
    query<{ uri: string }>(
      "local",
      "index",
      "SELECT uri FROM chat_messages WHERE uri = ?",
      [uri],
    ),
  ).toHaveLength(1);
  await sendChat(page, `upstream write after reload ${Date.now()}`);

  // Restart the actual local Go process, preserving its genuine on-disk
  // sessions. Control belongs to streamplace e2e stdin, not production HTTP.
  const restartFile = join(process.env.E2E_LOCAL_DATA_DIR!, "restart.json");
  const generation = existsSync(restartFile)
    ? JSON.parse(readFileSync(restartFile, "utf8")).generation
    : 0;
  writeFileSync(process.env.E2E_CONTROL_FIFO!, "restart-local\n");
  await expect
    .poll(
      () =>
        existsSync(restartFile)
          ? JSON.parse(readFileSync(restartFile, "utf8")).generation
          : 0,
      { timeout: 120_000 },
    )
    .toBe(generation + 1);
  expect(JSON.parse(readFileSync(restartFile, "utf8")).error).toBeUndefined();
  await page.reload();
  await expect(page.getByText(message).first()).toBeVisible();
  await sendChat(page, `upstream write after node restart ${Date.now()}`);

  // signOut sends a real DPoP-bound /oauth/revoke. Local downstream revocation
  // must not expose or destroy the station's long-lived confidential PDS grant.
  await page.goto(`${LOCAL}/settings/account`);
  const cancellationsBefore = stripVTControlCharacters(
    readFileSync(process.env.E2E_ATMOQ_LOG!, "utf8"),
  )
    .split("\n")
    .filter(
      (line) =>
        line.includes(`track=${DID}`) && line.includes("subscribed cancelled"),
    ).length;
  const revoked = page.waitForResponse(
    (response) =>
      response.request().method() === "POST" &&
      response.url() === `${LOCAL}/oauth/revoke` &&
      response.ok(),
  );
  await page.getByText("Log out", { exact: true }).last().click();
  expect((await revoked).ok()).toBe(true);
  await expect
    .poll(() => sessions("local", jkt)[0]?.downstream_revoked_at)
    .toBeTruthy();
  expect(sessions("station", stationJkt)[0]?.downstream_revoked_at).toBeNull();
  await expect
    .poll(
      () =>
        stripVTControlCharacters(
          readFileSync(process.env.E2E_ATMOQ_LOG!, "utf8"),
        )
          .split("\n")
          .filter(
            (line) =>
              line.includes(`track=${DID}`) &&
              line.includes("subscribed cancelled"),
          ).length,
    )
    .toBeGreaterThan(cancellationsBefore);
  assertScopedIndex();
});

function assertScopedIndex(): void {
  expect(
    query("local", "index", "SELECT did FROM repos WHERE did = ?", [UNRELATED]),
  ).toEqual([]);
  for (const entry of pdsAudit()) {
    expect(entry.path, "local node must not enumerate PDS accounts").not.toBe(
      "/xrpc/com.atproto.sync.listRepos",
    );
    expect(
      entry.path,
      "local node must not fall back to PDS WebSocket",
    ).not.toBe("/xrpc/com.atproto.sync.subscribeRepos");
    const values: string[] = [];
    new URLSearchParams(entry.query).forEach((value) => values.push(value));
    expect(values).not.toContain(UNRELATED);
    expect(values).not.toContain(process.env.UNRELATED_ACCOUNT_HANDLE);
  }
  const relayLog = stripVTControlCharacters(
    readFileSync(process.env.E2E_ATMOQ_LOG!, "utf8"),
  );
  expect(relayLog).toContain(`track=${DID}`);
  expect(relayLog).not.toMatch(/\btrack=(?:"atproto"|atproto)(?:\s|$)/);
  expect(relayLog).not.toContain(`track=${UNRELATED}`);
}
