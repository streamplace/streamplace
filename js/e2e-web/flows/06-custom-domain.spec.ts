import { expect, test } from "@playwright/test";
import { readFileSync } from "node:fs";
import { request } from "node:http";
import { join } from "node:path";
import { logInWithOAuth } from "../oauth";
import { pointAppAtNode } from "../server-setup";
import { zipDir } from "../zip";

// Custom domains: one node, several hostnames, each branded by its owner's
// place.stream.branding.brand record. The harness grants CUSTOM_DOMAINS
// (default "localhost") to the test account before the node starts, and
// http://localhost:<port> reaches the same node as SERVER_URL
// (http://127.0.0.1:<port>) under that other hostname.

const SERVER_URL = process.env.SERVER_URL!;
const PDS_URL = process.env.PDS_URL;
const DID = process.env.ACCOUNT_DID!;
const DOMAIN = (process.env.CUSTOM_DOMAINS ?? "").split(",")[0];
const BRAND = "place.stream.branding.brand";

const port = () => new URL(SERVER_URL).port;
const on = (host: string) => `http://${host}:${port()}`;

async function pds(path: string, init: RequestInit & { jwt?: string } = {}) {
  const headers: Record<string, string> = {
    ...(init.headers as Record<string, string>),
  };
  if (init.jwt) headers.authorization = `Bearer ${init.jwt}`;
  const res = await fetch(`${PDS_URL}/xrpc/${path}`, { ...init, headers });
  const body = await res.json();
  if (!res.ok)
    throw new Error(`${path}: ${res.status} ${JSON.stringify(body)}`);
  return body;
}

// GET path from the node as if it were reached at host: the harness node
// on 127.0.0.1 with that Host header (Node, unlike Chromium, does not
// resolve *.localhost).
function getOn(
  host: string,
  path: string,
): Promise<{ status: number; body: Buffer }> {
  return new Promise((resolve, reject) => {
    const req = request(
      {
        host: "127.0.0.1",
        port: port(),
        path,
        headers: { host: `${host}:${port()}` },
      },
      (res) => {
        const chunks: Buffer[] = [];
        res.on("data", (c) => chunks.push(c));
        res.on("end", () =>
          resolve({ status: res.statusCode!, body: Buffer.concat(chunks) }),
        );
      },
    );
    req.on("error", reject);
    req.end();
  });
}

// The branding a node serves on a hostname, as { key: text }.
async function brandingOn(host: string): Promise<Record<string, string>> {
  const res = await getOn(host, "/xrpc/place.stream.branding.getBranding");
  expect(res.status).toBe(200);
  const { assets } = JSON.parse(res.body.toString());
  return Object.fromEntries(
    assets.map((a: { key: string; data?: string }) => [a.key, a.data ?? ""]),
  );
}

test.describe("06-custom-domain", () => {
  test.skip(!DOMAIN || !PDS_URL, "harness started without a custom domain");

  test("a brand record the owner publishes styles their custom domain", async ({
    page,
  }) => {
    // Any atproto client can publish the brand: here, plain XRPC against
    // the owner's PDS with a password session, no Streamplace involved.
    const session = await pds("com.atproto.server.createSession", {
      method: "POST",
      headers: { "content-type": "application/json" },
      body: JSON.stringify({
        identifier: process.env.ACCOUNT_HANDLE,
        password: process.env.ACCOUNT_PASSWORD,
      }),
    });
    const mark = readFileSync(join(__dirname, "../fixtures/brand/mark.svg"));
    const { blob } = await pds("com.atproto.repo.uploadBlob", {
      method: "POST",
      jwt: session.accessJwt,
      headers: { "content-type": "image/svg+xml" },
      body: mark,
    });
    const title = `E2E Custom Domain ${Date.now()}`;
    await pds("com.atproto.repo.putRecord", {
      method: "POST",
      jwt: session.accessJwt,
      headers: { "content-type": "application/json" },
      body: JSON.stringify({
        repo: DID,
        collection: BRAND,
        rkey: DOMAIN,
        record: {
          $type: BRAND,
          siteTitle: title,
          primaryColor: "#E11D48",
          mainLogo: blob,
          appName: "E2E Custom App",
        },
      }),
    });

    // The node follows the record off the firehose.
    await expect
      .poll(async () => (await brandingOn(DOMAIN)).siteTitle, {
        timeout: 60_000,
      })
      .toBe(title);
    const branding = await brandingOn(DOMAIN);
    expect(branding.primaryColor).toBe("#e11d48"); // normalized on the way in
    expect(branding).not.toHaveProperty("appName"); // build-time only
    const logo = await getOn(
      DOMAIN,
      "/xrpc/place.stream.branding.getBlob?key=mainLogo",
    );
    expect(logo.body).toEqual(mark);

    // The node's own hostname keeps the node's brand.
    expect((await brandingOn("127.0.0.1")).siteTitle).not.toBe(title);

    // And the app on the custom domain wears it, from the first paint.
    await pointAppAtNode(page, on(DOMAIN));
    await page.goto(`${on(DOMAIN)}/`);
    await expect(page).toHaveTitle(title);
    await expect(page.getByText(title).first()).toBeVisible();
    await page.goto(`${SERVER_URL}/`);
    await expect(page.getByText(title)).toHaveCount(0);
  });

  test("an admin grants a domain and its owner imports a brand directory into it", async ({
    page,
  }) => {
    test.skip(
      !process.env.SERVER_HTTPS_URL,
      "harness started without its HTTPS hostnames (no OAuth)",
    );
    const httpsURL = process.env.SERVER_HTTPS_URL!;
    const host = "brand.localhost";

    await logInWithOAuth(page);
    await page.goto(`${httpsURL}/settings/branding`);

    // Grant: the test account is a node admin, and owns the domain it adds.
    const hostField = page
      .locator(
        '[data-testid="branding-domain-hostname"], [data-testid="branding-domain-hostname"] input',
      )
      .first();
    await expect(hostField).toBeVisible({ timeout: 30_000 });
    await hostField.fill(host);
    await page.getByTestId("branding-domain-add").click();
    await expect(page.getByTestId(`branding-domain-${host}`)).toBeVisible();

    // Point the rest of the screen at the domain's brand, then import the
    // same kind of brand directory an app build takes (SP_BRAND_DIR).
    await page.getByTestId(`branding-domain-edit-${host}`).click();
    await expect(
      page
        .locator(
          '[data-testid="branding-broadcaster-did"], [data-testid="branding-broadcaster-did"] input',
        )
        .first(),
    ).toHaveValue(`did:web:${host}`);
    const chooser = page.waitForEvent("filechooser");
    await page.getByTestId("branding-bundle-import").click();
    await (
      await chooser
    ).setFiles({
      name: "brand.zip",
      mimeType: "application/zip",
      buffer: zipDir(join(__dirname, "../fixtures/brand")),
    });
    await page.getByTestId("branding-bundle-apply").click();

    // The import was published as the owner's brand record, build-time
    // keys and all, so an app build can pull the whole brand from it...
    await expect
      .poll(
        async () => {
          const res = await fetch(
            `${PDS_URL}/xrpc/com.atproto.repo.getRecord?repo=${DID}&collection=${BRAND}&rkey=${host}`,
          );
          return res.ok ? (await res.json()).value : null;
        },
        { timeout: 30_000 },
      )
      .toMatchObject({
        siteTitle: "E2E Imported Brand",
        appName: "E2E Imported App",
        appBundleId: "place.stream.e2e.imported",
        appColors: { iconBackground: "#0ea5e9" },
        mainLogo: { $type: "blob", mimeType: "image/svg+xml" },
      });

    // ...and the domain serves it.
    expect((await brandingOn(host)).siteTitle).toBe("E2E Imported Brand");
    await pointAppAtNode(page, on(host));
    await page.goto(`${on(host)}/`);
    await expect(page).toHaveTitle("E2E Imported Brand");
  });
});
