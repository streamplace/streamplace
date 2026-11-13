import { cp, mkdir, readFile, stat } from "node:fs/promises";
import { join, resolve } from "node:path";
import { expect, test } from "./fixture";
import { loginThroughSystemBrowser } from "./login";

// Also Desktop's lifecycle check: the session and PDS writes survive a restart.
test("OBS gets a key through Desktop login, kept across reloads and restarts", async ({
  desktop,
  request,
}) => {
  test.skip(process.platform !== "linux", "Uses the Linux OBS config layout");
  const app = desktop.electronApp();
  const configHome = await app.evaluate(() => process.env.XDG_CONFIG_HOME!);
  const config = join(configHome, "obs-studio");
  const plugin = join(config, "plugins", "obs-streamplace", "data");
  await mkdir(plugin, { recursive: true });
  await cp(
    resolve(
      __dirname,
      "../../desktop/out/obs-plugin",
      `linux-${process.arch === "x64" ? "amd64" : process.arch}`,
      "obs-streamplace/data/manifest.json",
    ),
    join(plugin, "manifest.json"),
  );
  const handleField = desktop.page
    .locator('[data-testid="login-handle"], [data-testid="login-handle"] input')
    .first();
  await expect(handleField).toBeVisible({ timeout: 60_000 });
  await desktop.page.keyboard.press("Escape");
  await expect(handleField).not.toBeVisible();
  expect((await request.get(`${desktop.nodeURL}/desktop/obs`)).ok()).toBe(true);
  await loginThroughSystemBrowser(desktop);

  const file = join(
    config,
    "plugin_config",
    "obs-streamplace",
    "stream-key.json",
  );
  await expect
    .poll(async () => readFile(file, "utf8").catch(() => ""), {
      timeout: 60_000,
    })
    .toContain(process.env.ACCOUNT_DID!);
  const original = await readFile(file, "utf8");
  expect(JSON.parse(original)).toEqual({
    did: process.env.ACCOUNT_DID,
    key: expect.stringMatching(/^z[1-9A-HJ-NP-Za-km-z]+$/),
    replaces: [],
  });
  const modified = (await stat(file)).mtimeMs;
  await expect(desktop.page.getByTestId("desktop-obs-connected")).toBeVisible();
  await expect(
    desktop.page.getByRole("button", { name: "Connect OBS", exact: true }),
  ).toBeEnabled();
  await desktop.page.reload();
  await expect(desktop.page.getByTestId("desktop-obs-connected")).toBeVisible();
  await expect(
    desktop.page.getByRole("button", { name: "Connect OBS", exact: true }),
  ).toBeEnabled();
  expect(await readFile(file, "utf8")).toBe(original);
  expect((await stat(file)).mtimeMs).toBe(modified);

  // An explicit request still re-keys an already connected account.
  expect((await request.get(`${desktop.nodeURL}/desktop/obs`)).ok()).toBe(true);
  await expect
    .poll(() => readFile(file, "utf8"), { timeout: 60_000 })
    .not.toBe(original);
  expect(JSON.parse(await readFile(file, "utf8"))).toEqual({
    did: process.env.ACCOUNT_DID,
    key: expect.stringMatching(/^z[1-9A-HJ-NP-Za-km-z]+$/),
    replaces: [JSON.parse(original).key],
  });

  const keyRecords = async () => {
    const url = new URL(
      "/xrpc/com.atproto.repo.listRecords",
      process.env.PDS_HTTPS_URL!,
    );
    url.search = new URLSearchParams({
      repo: process.env.ACCOUNT_DID!,
      collection: "place.stream.key",
    }).toString();
    const response = await request.get(url.toString(), {
      ignoreHTTPSErrors: true,
    });
    expect(response.ok()).toBe(true);
    return ((await response.json()).records as unknown[]).length;
  };
  const rekeyed = await readFile(file, "utf8");
  const before = await keyRecords();
  const page = await desktop.restart();
  await expect(
    page.getByText(`Signed in as @${process.env.ACCOUNT_HANDLE}`),
  ).toBeVisible({ timeout: 60_000 });
  await expect(page.getByTestId("desktop-obs-connected")).toBeVisible();
  expect(await readFile(file, "utf8")).toBe(rekeyed);
  expect(await keyRecords()).toBe(before);
  // The session still writes to the PDS after a restart.
  await page.getByRole("button", { name: "Connect OBS", exact: true }).click();
  await expect.poll(keyRecords, { timeout: 60_000 }).toBe(before + 1);
});
