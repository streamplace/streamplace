import { expect, test, type Locator, type Page } from "@playwright/test";

// Vite web beta only: these DOM controls are not the Expo/native player.
// Run against the real looping harness stream, opting in through its beta cookie.
// Each viewport exercises both chat states and fullscreen, not a single action.
test.setTimeout(180_000);

const viewports = [
  { width: 320, height: 568 },
  { width: 390, height: 844 },
  // 384px player-content breakpoint, plus the control bar's 24px padding.
  { width: 407, height: 844 },
  { width: 408, height: 844 },
  { width: 844, height: 390 },
  // The web layout switches at a square viewport, not a width breakpoint.
  { width: 799, height: 800 },
  { width: 800, height: 800 },
  { width: 801, height: 800 },
  { width: 1023, height: 720 },
  { width: 1024, height: 720 },
  { width: 1080, height: 720 },
  { width: 1080, height: 1200 },
  { width: 1080, height: 1080 },
  { width: 1280, height: 800 },
  { width: 1440, height: 900 },
  { width: 1920, height: 1080 },
];

async function expectUsable(control: Locator) {
  await expect(control).toBeVisible({ timeout: 5_000 });
  await expect(control).toBeInViewport({ ratio: 1 });
  await expect(control).toBeEnabled();
  await expect
    .poll(() =>
      control.evaluate((element) => {
        let opacity = 1;
        for (
          let parent: Element | null = element;
          parent;
          parent = parent.parentElement
        ) {
          opacity *= Number(getComputedStyle(parent).opacity);
        }
        const box = element.getBoundingClientRect();
        const hit = document.elementFromPoint(
          box.x + box.width / 2,
          box.y + box.height / 2,
        );
        return (
          opacity > 0.99 &&
          !!hit &&
          (element.contains(hit) || hit.contains(element))
        );
      }),
    )
    .toBe(true);
}

async function revealControls(page: Page) {
  const video = page.locator("video").first();
  await expect(video).toBeVisible();
  await video.locator("..").hover();
}

for (const viewport of viewports) {
  test(`player viewport: ${viewport.width}x${viewport.height}, chat open and closed`, async ({
    page,
    context,
    baseURL,
  }) => {
    if (!baseURL || !process.env.ACCOUNT_DID)
      throw new Error("harness URL and ACCOUNT_DID required");
    await page.setViewportSize(viewport);
    await context.addCookies([
      { name: "sp_web_beta", value: "1", url: baseURL },
    ]);
    // Layout is shared across transports; use standard HLS to avoid WebRTC
    // reconnect transitions while inspecting the paused controls.
    await page.addInitScript(() => {
      localStorage.setItem("player-transport", "hls");
    });
    // The isolated harness account isn't indexed by the public Bluesky AppView.
    // Only identity discovery is substituted; stream, chat and media remain real.
    await page.route(
      `https://public.api.bsky.app/xrpc/app.bsky.actor.getProfile?actor=${encodeURIComponent(process.env.ACCOUNT_DID)}`,
      (route) =>
        route.fulfill({
          headers: { "Access-Control-Allow-Origin": "*" },
          json: {
            did: process.env.ACCOUNT_DID,
            handle: process.env.ACCOUNT_HANDLE,
          },
        }),
    );
    await page.goto(`/${process.env.ACCOUNT_DID}`);
    const video = page.locator("video").first();
    const player = video.locator("..");
    await expect
      .poll(() =>
        video.evaluate((element: HTMLVideoElement) => element.currentTime),
      )
      .toBeGreaterThan(0);
    // Prove real playback first, then hold the chrome through the normal pause
    // shortcut: layout inspection must not race idle hiding or live buffering.
    await page.keyboard.press("Space");
    await expect
      .poll(() => video.evaluate((element: HTMLVideoElement) => element.paused))
      .toBe(true);

    for (const chatOpen of [true, false]) {
      await test.step(`chat ${chatOpen ? "open" : "closed"}`, async () => {
        await revealControls(page);
        await expect(
          page
            .getByRole("button", {
              name: chatOpen ? "Close chat" : "Open chat",
              exact: true,
            })
            .first(),
        ).toBeVisible();
        await expectUsable(
          player.locator('button[title="Pause"], button[title="Play"]'),
        );
        // The centered autoplay prompt has the same label; the control bar mute
        // button also has a title, so distinguish the actual bar control.
        await expectUsable(
          player.locator('button[title="Unmute"], button[title="Mute"]'),
        );
        await expectUsable(
          player.getByRole("button", { name: "Settings", exact: true }),
        );
        await expectUsable(
          player.getByRole("button", { name: "Fullscreen", exact: true }),
        );
        await expectUsable(
          player.getByRole("button", {
            name: "Picture-in-picture",
            exact: true,
          }),
        );
        const playerWidth = (await player.boundingBox())!.width;
        const volume = player.locator(
          '[data-slot="slider"][aria-label="Volume"]',
        );
        // Expanded chrome needs 384px of content width plus 24px padding.
        // Desktop-sized viewports must expose volume even with chat open.
        if (viewport.width >= 1024 || playerWidth >= 408) {
          await expectUsable(volume);
          await expectUsable(
            player.getByRole("button", { name: "Theatre mode", exact: true }),
          );
          await expectUsable(
            player.getByRole("button", { name: "Enable danmu", exact: true }),
          );
          // A different target after closing chat must produce a fresh update.
          const targetVolume = chatOpen ? 0.25 : 0.75;
          const sliderBox = (await volume.boundingBox())!;
          await volume.click({
            position: {
              x: sliderBox.width * targetVolume,
              y: sliderBox.height / 2,
            },
          });
          await expect
            .poll(() =>
              video.evaluate((element: HTMLVideoElement) => element.volume),
            )
            .toBeCloseTo(targetVolume, 1);
          await expect
            .poll(() =>
              video.evaluate((element: HTMLVideoElement) => element.muted),
            )
            .toBe(false);
        } else {
          await expect(volume).toBeHidden();
        }
        await player
          .getByRole("button", { name: "Settings", exact: true })
          .click();
        await expect(
          page.getByRole("menuitemradio", { name: /Standard/ }),
        ).toBeVisible();
        await expect(
          page.getByRole("menuitemcheckbox", {
            name: "Theatre mode",
            exact: true,
          }),
        ).toBeVisible();
        await expect(
          page.getByRole("menuitemcheckbox", { name: "Danmu", exact: true }),
        ).toBeVisible();
        await page.keyboard.press("Escape");
        if (chatOpen) {
          await page
            .getByRole("button", { name: "Close chat", exact: true })
            .last()
            .click();
          await expect(
            page.getByRole("button", { name: "Open chat", exact: true }),
          ).toBeVisible();
        }
      });
    }
    await revealControls(page);
    await player
      .getByRole("button", { name: "Fullscreen", exact: true })
      .click();
    await expect
      .poll(() => page.evaluate(() => !!document.fullscreenElement))
      .toBe(true);
    await expectUsable(
      player.getByRole("button", { name: "Exit fullscreen", exact: true }),
    );
    await player
      .getByRole("button", { name: "Exit fullscreen", exact: true })
      .click();
    await expect
      .poll(() => page.evaluate(() => !!document.fullscreenElement))
      .toBe(false);
  });
}
