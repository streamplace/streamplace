import { cp, mkdir, readFile, writeFile } from "node:fs/promises";
import { join, resolve } from "node:path";
import { expect, test } from "./fixture";
import { loginThroughSystemBrowser } from "./login";

// Also Desktop's lifecycle check: the login and OBS connection survive a restart.
test("OBS connects to Desktop's internal ingest and stays connected after restart", async ({
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
  const profile = join(config, "basic", "profiles", "Desktop");
  await mkdir(profile, { recursive: true });
  await writeFile(join(config, "user.ini"), "[Basic]\nProfileDir=Desktop\n");
  await writeFile(join(profile, "basic.ini"), "[General]\nName=Desktop\n");
  // A Streamplace profile aimed at another node isn't connected to this one.
  await writeFile(
    join(profile, "service.json"),
    JSON.stringify({
      type: "rtmp_common",
      settings: {
        service: "Streamplace",
        server: "http://127.0.0.1:38080",
        key: "streamplace-desktop",
      },
    }),
  );

  const handleField = desktop.page
    .locator('[data-testid="login-handle"], [data-testid="login-handle"] input')
    .first();
  await expect(handleField).toBeVisible({ timeout: 60_000 });
  // RN-web's Modal only handles Escape once it has become active after mounting.
  await expect(async () => {
    await desktop.page.keyboard.press("Escape");
    await expect(handleField).not.toBeVisible({ timeout: 1_000 });
  }).toPass();
  await expect(desktop.page.getByTestId("desktop-login")).toBeVisible();
  expect((await request.get(`${desktop.nodeURL}/desktop/obs`)).ok()).toBe(true);
  await expect(handleField).toBeVisible();
  await loginThroughSystemBrowser(desktop);
  await expect(
    desktop.page.getByRole("button", { name: "Connect OBS", exact: true }),
  ).toBeVisible();
  await expect(desktop.page.getByTestId("desktop-obs-connected")).toHaveCount(
    0,
  );

  await desktop.page
    .getByRole("button", { name: "Connect OBS", exact: true })
    .click();
  const file = join(profile, "service.json");
  const expected = {
    type: "rtmp_common",
    settings: {
      service: "Streamplace",
      server: desktop.internalURL,
      key: "streamplace-desktop",
    },
  };
  await expect
    .poll(async () =>
      JSON.parse(await readFile(file, "utf8").catch(() => "null")),
    )
    .toEqual(expected);
  await expect(desktop.page.getByTestId("desktop-obs-connected")).toBeVisible();

  const page = await desktop.restart();
  await expect(
    page.getByText(`Signed in as @${process.env.ACCOUNT_HANDLE}`),
  ).toBeVisible({ timeout: 60_000 });
  await expect(page.getByTestId("desktop-obs-connected")).toBeVisible();
  expect(JSON.parse(await readFile(file, "utf8"))).toEqual(expected);
});
