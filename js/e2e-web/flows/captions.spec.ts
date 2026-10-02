import { expect, test, type Page } from "@playwright/test";
import { loginThroughPds } from "./login";

// Live CC and pushed caption text mirror .maestro/logged-out/captions.yaml; the
// VOD, styling, and dashboard checks are web-only.

const DID = process.env.ACCOUNT_DID!;
const VIDEO_URI = process.env.VIDEO_URI!;
const STREAM_KEY = process.env.STREAM_KEY;
const HTTPS_URL = process.env.SERVER_HTTPS_URL;

async function ccOn(page: Page): Promise<boolean> {
  const button = page.getByTestId("player-cc-button").first();
  return (await button.getAttribute("aria-pressed")) === "true";
}
async function reveal(page: Page) {
  await page.locator("video").first().hover({ force: true });
  await expect(page.getByTestId("player-cc-button").first()).toBeVisible();
}

for (const frontend of ["app", "web"] as const) {
  test.describe(`captions: ${frontend}`, () => {
    test.beforeEach(async ({ context, baseURL }) => {
      for (const url of [baseURL!, HTTPS_URL].filter(Boolean) as string[]) {
        await context.addCookies([
          { name: "sp_web_beta", value: frontend === "web" ? "1" : "0", url },
        ]);
      }
    });
    test("CC toggles on live and VOD and persists between players", async ({
      page,
    }) => {
      await page.goto(`/${DID}`);
      await reveal(page);
      await expect(
        page.getByTestId("player-cc-button").first(),
      ).toHaveAttribute("aria-pressed", /^(true|false)$/);
      if (await ccOn(page))
        await page.getByTestId("player-cc-button").first().click();
      await page.getByTestId("player-cc-button").first().click();
      await expect.poll(() => ccOn(page)).toBe(true);
      const parts = VIDEO_URI.split("/");
      await page.goto(`/${parts[2]}/video/${parts[4]}`);
      await reveal(page);
      await expect.poll(() => ccOn(page)).toBe(true);
      await page.getByTestId("player-cc-button").first().click();
      await expect.poll(() => ccOn(page)).toBe(false);
      await page.reload();
      await reveal(page);
      await expect.poll(() => ccOn(page)).toBe(false);
    });
    test("real pushed live captions reach the overlay", async ({
      page,
      request,
      baseURL,
    }) => {
      test.skip(!STREAM_KEY, "harness did not export its stream key");
      await page.goto(`/${DID}`);
      await reveal(page);
      if (!(await ccOn(page)))
        await page.getByTestId("player-cc-button").first().click();
      const text = `Caption proof ${frontend} ${Date.now()}`;
      await expect(async () => {
        const now = Date.now();
        const response = await request.post(
          `${baseURL}/xrpc/place.stream.caption.pushCaptions`,
          {
            headers: { Authorization: `Bearer ${STREAM_KEY}` },
            data: {
              streamer: DID,
              language: "en",
              source: "human",
              cues: [
                {
                  id: `proof-${now}`,
                  startTime: new Date(now).toISOString(),
                  endTime: new Date(now + 4000).toISOString(),
                  text,
                  final: true,
                },
              ],
            },
          },
        );
        expect(response.ok(), await response.text()).toBe(true);
        const listed = await request.get(
          `${baseURL}/xrpc/place.stream.caption.listTracks`,
          { params: { streamer: DID } },
        );
        expect(listed.ok(), await listed.text()).toBe(true);
        const human = (await listed.json()).tracks.find(
          (track: { source: string }) => track.source === "human",
        );
        expect(human).toBeDefined();
        await reveal(page);
        await page.getByTestId("player-cc-menu-button").first().click();
        await page.getByTestId(`player-cc-track-${human.id}`).first().click();
        await page.keyboard.press("Escape");
        await expect(
          page.getByTestId("caption-overlay-text").first(),
        ).toContainText(text, { timeout: 3000 });
      }).toPass({ timeout: 60000, intervals: [1000] });
      await reveal(page);
      await page.getByTestId("player-cc-button").first().click();
      await expect(page.getByTestId("caption-overlay-text")).toHaveCount(0);
    });
    test("caption style survives reload with the preview", async ({ page }) => {
      await page.goto("/settings/captions");
      const previewText = page
        .getByTestId("settings-captions-preview")
        .getByTestId("caption-overlay-text");
      await page.getByTestId("settings-captions-size-100").click();
      const previewFontSize = (element: Element) => {
        const spans = element.querySelectorAll("span");
        return parseFloat(
          getComputedStyle(spans.item(spans.length - 1) || element).fontSize,
        );
      };
      const baselineSize = await previewText.evaluate(previewFontSize);
      await page.getByTestId("settings-captions-size-150").click();
      await page.getByTestId("settings-captions-text-color-yellow").click();
      await page.getByTestId("settings-captions-background-opacity-50").click();
      await page.reload();
      for (const id of [
        "size-150",
        "text-color-yellow",
        "background-opacity-50",
      ])
        await expect(
          page.getByTestId(`settings-captions-${id}`),
        ).toHaveAttribute("aria-checked", "true");
      await expect(page.getByTestId("settings-captions-preview")).toBeVisible();
      expect(await previewText.evaluate(previewFontSize)).toBeCloseTo(
        baselineSize * 1.5,
        0,
      );
    });
    test("imports VOD captions, downloads both formats, and renders them", async ({
      page,
    }) => {
      test.skip(frontend !== "web", "modern web VOD management surface");
      test.skip(!HTTPS_URL, "OAuth requires the HTTPS harness");
      await loginThroughPds(page);
      await page.goto(
        `${HTTPS_URL}/dashboard/videos?video=${encodeURIComponent(VIDEO_URI)}`,
      );
      const text = `VOD caption proof ${Date.now()}`;
      await page.getByTestId("vod-captions-language").fill("fr");
      const imported = page.waitForResponse(
        (response) =>
          response
            .url()
            .includes("/xrpc/place.stream.caption.importCaptions") &&
          response.request().method() === "POST",
      );
      await page.getByTestId("vod-captions-upload").setInputFiles({
        name: "proof.vtt",
        mimeType: "text/vtt",
        buffer: Buffer.from(`WEBVTT\n\n00:00.000 --> 01:00.000\n${text}\n`),
      });
      const response = await imported;
      expect(response.ok(), await response.text()).toBe(true);
      const listed = await page.evaluate(
        async ({ url, video }) => {
          const result = await fetch(
            `${url}/xrpc/place.stream.caption.listTracks?video=${encodeURIComponent(video)}`,
          );
          return { ok: result.ok, body: await result.text() };
        },
        { url: HTTPS_URL!, video: VIDEO_URI },
      );
      expect(listed.ok, listed.body).toBe(true);
      const track = JSON.parse(listed.body).tracks.find(
        (track: { source: string; language: string }) =>
          track.source === "imported" && track.language === "fr",
      );
      expect(track).toBeDefined();
      const vtt = page.getByTestId(`vod-captions-download-${track.id}-vtt`);
      await expect(vtt).toBeVisible();
      for (const format of ["vtt", "srt"]) {
        const link = page.getByTestId(
          `vod-captions-download-${track.id}-${format}`,
        );
        const [download] = await Promise.all([
          page.waitForEvent("download"),
          link.click(),
        ]);
        const stream = await download.createReadStream();
        expect(stream).not.toBeNull();
        let body = "";
        for await (const chunk of stream!) body += chunk.toString();
        expect(body).toContain(text);
        if (format === "vtt") expect(body).toMatch(/^WEBVTT/);
        else expect(body).toMatch(/\d{2}:\d{2}:\d{2},\d{3} -->/);
      }
      const parts = VIDEO_URI.split("/");
      await page.goto(`${HTTPS_URL}/${parts[2]}/video/${parts[4]}`);
      await reveal(page);
      await page.getByTestId("player-cc-menu-button").first().click();
      await page.getByTestId(`player-cc-track-${track.id}`).first().click();
      await page.keyboard.press("Escape");
      await expect(
        page.getByTestId("caption-overlay-text").first(),
      ).toContainText(text);
    });
    test("dashboard saves canonical off and auto", async ({ page }) => {
      test.skip(!HTTPS_URL, "OAuth requires the HTTPS harness");
      await loginThroughPds(page);
      await page.goto(
        `${HTTPS_URL}${frontend === "web" ? "/dashboard/stream" : "/live"}`,
      );
      if (frontend === "app")
        await page.getByText("Metadata", { exact: true }).first().click();
      const toggle =
        frontend === "app"
          ? page.getByTestId("dashboard-captions-auto").getByRole("switch")
          : page.getByTestId("dashboard-captions-auto");
      await expect(toggle).toBeVisible();
      for (const canonical of ["off", "auto"]) {
        const checked =
          frontend === "app"
            ? await toggle.isChecked()
            : (await toggle.getAttribute("aria-checked")) === "true";
        if (checked !== (canonical === "auto")) await toggle.click();
        const saved = page.waitForResponse(
          (response) =>
            response.request().method() === "POST" &&
            response.url().includes("/xrpc/com.atproto.repo.putRecord") &&
            response
              .request()
              .postData()
              ?.includes("place.stream.metadata.configuration") === true,
        );
        await page.getByTestId("dashboard-metadata-save").click();
        const response = await saved;
        expect(response.ok(), await response.text()).toBe(true);
        expect(
          response.request().postDataJSON().record.captionPolicy.canonical,
        ).toBe(canonical);
        await page.reload();
        if (frontend === "app")
          await page.getByText("Metadata", { exact: true }).first().click();
        if (frontend === "app")
          await expect(toggle).toBeChecked({ checked: canonical === "auto" });
        else
          await expect(toggle).toHaveAttribute(
            "aria-checked",
            String(canonical === "auto"),
          );
      }
    });
  });
}
