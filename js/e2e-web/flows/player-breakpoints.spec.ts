import { expect, test } from "@playwright/test";

// Web-only: native live playback uses the device's hardware volume controls.
// Cover both sides of theme, sidebar (981), and desktop-player (1201) boundaries.
const widths = [
  1080, 320, 639, 640, 641, 767, 768, 769, 980, 981, 982, 1023, 1024, 1025,
  1199, 1200, 1201, 1279, 1280, 1281, 1535, 1536, 1537,
];

for (const orientation of ["portrait", "landscape"] as const) {
  test(`player-breakpoints: ${orientation} resize boundaries`, async ({
    page,
  }, testInfo) => {
    test.setTimeout(600_000);
    const viewport = (width: number) => ({
      width,
      height:
        orientation === "portrait"
          ? width + 200
          : width === 1080
            ? 722
            : Math.min(568, Math.round((width * 3) / 4)),
    });
    await page.setViewportSize(viewport(widths[0]));
    await page.goto("/");
    await page.getByTestId("home-stream-card").first().click();
    const video = page.locator("video").first();

    const revealControls = async () => {
      await page.mouse.move(0, 0);
      // Resizing can remount the video. Wait for its replacement's geometry;
      // the player overlay, not the video element, receives the pointer.
      await expect(async () => {
        const box = await video.boundingBox();
        if (!box) throw new Error("player video has no bounding box");
        await page.mouse.move(box.x + box.width / 2, box.y + box.height / 2);
      }).toPass({ timeout: 30_000 });
      await expect
        .poll(() =>
          video.evaluate((element: HTMLVideoElement) => element.readyState),
        )
        .toBeGreaterThanOrEqual(2);
    };

    const exerciseVolume = async () => {
      await revealControls();
      const mute = page.getByRole("button", { name: "Mute", exact: true });
      await expect(mute).toBeInViewport({ ratio: 1 });
      await mute.click({ timeout: 5_000 });
      await expect
        .poll(() =>
          video.evaluate((element: HTMLVideoElement) => element.muted),
        )
        .toBe(true);
      const unmute = page.getByRole("button", {
        name: "Unmute",
        exact: true,
      });
      await expect(unmute).toBeInViewport({ ratio: 1 });
      await unmute.hover();
      const slider = page.getByRole("slider");
      await expect(slider).toBeInViewport({ ratio: 1 });
      await expect(page.getByTestId("player-volume-track")).toBeInViewport({
        ratio: 1,
      });
      await unmute.click();
      await expect
        .poll(() =>
          video.evaluate((element: HTMLVideoElement) => element.muted),
        )
        .toBe(false);
      await slider.focus();
      await slider.press("Home");
      await expect
        .poll(() =>
          video.evaluate((element: HTMLVideoElement) => element.muted),
        )
        .toBe(true);
      await slider.press("ArrowRight");
      await expect
        .poll(() =>
          video.evaluate((element: HTMLVideoElement) => element.volume),
        )
        .toBeGreaterThan(0);
      await expect
        .poll(() =>
          video.evaluate((element: HTMLVideoElement) => element.muted),
        )
        .toBe(false);
    };

    for (const width of widths) {
      const size = viewport(width);
      await test.step(`${size.width}x${size.height}`, async () => {
        await page.setViewportSize(size);
        await exerciseVolume();
        if (orientation === "landscape" && width >= 768) {
          const collapseChat = page.getByRole("button", {
            name: "Collapse chat",
            exact: true,
          });
          await expect(collapseChat).toBeInViewport();
          await collapseChat.click();
          await expect(collapseChat).not.toBeInViewport();
          await exerciseVolume();
          await revealControls();
          await page
            .getByRole("button", { name: "Show chat", exact: true })
            .click();
          await expect(collapseChat).toBeInViewport();
          await exerciseVolume();
        }
        if (width === 1080 && orientation === "landscape") {
          await revealControls();
          await testInfo.attach("1080x722-chat-open", {
            body: await page.screenshot(),
            contentType: "image/png",
          });
        }
      });
    }
  });
}
