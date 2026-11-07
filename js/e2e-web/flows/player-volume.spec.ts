import { expect, test } from "@playwright/test";

// Web-only: native live playback uses the device's hardware volume controls.
for (const width of [900, 1440]) {
  test(`player volume: pointer position and drag direction at ${width}px`, async ({
    page,
  }, testInfo) => {
    await page.setViewportSize({ width, height: 900 });
    await page.goto("/");
    await page.getByTestId("home-stream-card").first().click();
    const video = page.locator("video").first();
    await expect
      .poll(() =>
        video.evaluate((element: HTMLVideoElement) => element.currentTime),
      )
      .toBeGreaterThan(0);
    const videoBox = (await video.boundingBox())!;
    await page.mouse.move(
      videoBox.x + videoBox.width / 2,
      videoBox.y + videoBox.height / 2,
    );
    await page.getByRole("button", { name: "Mute", exact: true }).hover();
    const track = page.getByTestId("player-volume-track");
    const slider = page.getByRole("slider");
    await expect(track).toBeInViewport({ ratio: 1 });
    // Hover waits for the expanding track to become stable before measuring it.
    await track.hover();
    const box = (await track.boundingBox())!;
    expect(box.width).toBeGreaterThan(box.height);
    const y = box.y + box.height / 2;
    await page.mouse.click(box.x + box.width * 0.25, y);
    await expect
      .poll(() => video.evaluate((element: HTMLVideoElement) => element.volume))
      .toBeCloseTo(0.25 ** 2, 1);
    const thumb = (await slider.boundingBox())!;
    expect(thumb.x + thumb.width / 2).toBeGreaterThan(box.x);
    expect(thumb.x + thumb.width / 2).toBeLessThan(box.x + box.width / 2);
    expect(Math.abs(thumb.y + thumb.height / 2 - y)).toBeLessThan(1);
    const fill = (await track.locator(":scope > div").boundingBox())!;
    expect(fill.width / box.width).toBeCloseTo(0.25, 1);
    await page.mouse.move(thumb.x + thumb.width / 2, y);
    await page.mouse.down();
    await page.mouse.move(box.x + box.width * 0.75, y, { steps: 12 });
    await expect
      .poll(() => video.evaluate((element: HTMLVideoElement) => element.volume))
      .toBeCloseTo(0.75 ** 2, 1);
    // Hold a drag beyond the old idle timer; it must not collapse under us.
    await page.waitForTimeout(1800);
    await expect(slider).toBeInViewport({ ratio: 1 });
    await page.mouse.move(box.x + box.width * 0.25, y, { steps: 12 });
    await page.mouse.up();
    await expect
      .poll(() => video.evaluate((element: HTMLVideoElement) => element.volume))
      .toBeCloseTo(0.25 ** 2, 1);
    await expect
      .poll(() => video.evaluate((element: HTMLVideoElement) => element.muted))
      .toBe(false);
    await testInfo.attach("volume-quarter", {
      body: await page.screenshot(),
      contentType: "image/png",
    });
    await slider.press("Home");
    await expect
      .poll(() => video.evaluate((element: HTMLVideoElement) => element.muted))
      .toBe(true);
    await slider.press("End");
    await expect
      .poll(() => video.evaluate((element: HTMLVideoElement) => element.volume))
      .toBe(1);
    await expect
      .poll(() => video.evaluate((element: HTMLVideoElement) => element.muted))
      .toBe(false);
  });
}
