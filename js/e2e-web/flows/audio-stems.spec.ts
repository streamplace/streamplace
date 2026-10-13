import { expect, test, type Locator } from "@playwright/test";

// Mirror of .maestro/logged-out/audio-stems.yaml. Over fMP4 ingest the harness
// pushes the test stream with a stem labeled "Music" beside the full mix, and
// WebRTC playback receives both: the video element plays the full mix and each
// stem the viewer mixes in plays from its own audio element.
test.skip(
  process.env.SP_E2E_INGEST !== "fmp4",
  "the test stream has stems only over fMP4 ingest",
);

// Whether the video element's audio, the full mix, is audible.
const fullMixEnabled = (video: Locator) =>
  video.evaluate(
    (v: HTMLVideoElement) =>
      (v.srcObject as MediaStream | null)?.getAudioTracks()[0]?.enabled ?? null,
  );

test("audio stems: switch from the full mix to the stems and back", async ({
  page,
}) => {
  await page.goto("/");
  const card = page.getByTestId("home-stream-card").first();
  await expect(card).toBeVisible({ timeout: 30_000 });
  await card.click();

  const video = page.locator("video").first();
  await expect(video).toBeVisible({ timeout: 30_000 });
  await expect.poll(() => fullMixEnabled(video)).toBe(true);
  const stemAudio = page.getByTestId("player-stem-audio");
  await expect(stemAudio).toHaveCount(0);

  // moving the mouse onto the video reveals the player controls (the
  // player's overlay takes the pointer, so hover() can't target the video)
  const box = await video.boundingBox();
  if (!box) {
    throw new Error("player video has no bounding box");
  }
  await page.mouse.move(box.x + box.width / 2, box.y + box.height + 16);
  await page.mouse.move(box.x + box.width / 2, box.y + box.height / 2);
  await page.locator('[data-testid="player-menu"]:visible').first().click();
  await expect(page.getByText("Audio", { exact: true })).toBeVisible();
  const music = page.getByTestId("player-audio-stem-0");
  await expect(music).toContainText("Music");

  await music.click();
  await page.getByTestId("player-audio-stem-0-level-50").click();
  await expect(stemAudio).toHaveCount(1);
  await expect.poll(() => fullMixEnabled(video)).toBe(false);
  expect(await stemAudio.evaluate((a: HTMLAudioElement) => a.volume)).toBe(0.5);
  expect(
    await stemAudio.evaluate(
      (a: HTMLAudioElement) =>
        (a.srcObject as MediaStream).getAudioTracks()[0].readyState,
    ),
  ).toBe("live");
  await expect
    .poll(() => stemAudio.evaluate((a: HTMLAudioElement) => a.paused))
    .toBe(false);
  const startTime = await stemAudio.evaluate(
    (a: HTMLAudioElement) => a.currentTime,
  );
  await expect
    .poll(() => stemAudio.evaluate((a: HTMLAudioElement) => a.currentTime))
    .toBeGreaterThan(startTime + 0.1);

  // Pausing the video must also pause the separately rendered stem.
  await video.evaluate((v: HTMLVideoElement) => v.pause());
  await expect
    .poll(() => stemAudio.evaluate((a: HTMLAudioElement) => a.paused))
    .toBe(true);
  await video.evaluate((v: HTMLVideoElement) => v.play());
  await expect
    .poll(() => stemAudio.evaluate((a: HTMLAudioElement) => a.paused))
    .toBe(false);
  await expect(music).toContainText("50%");

  await page.getByTestId("player-audio-full-mix").click();
  await expect(stemAudio).toHaveCount(0);
  await expect.poll(() => fullMixEnabled(video)).toBe(true);
});
