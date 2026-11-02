import { expect, test } from "@playwright/test";
import { loginThroughPds } from "./login";

const url = process.env.SERVER_HTTPS_URL;
test.skip(!url, "harness started without its HTTPS hostnames");

test("health without a connection does not invent chart readings", async ({
  page,
}) => {
  await page.addInitScript(() => {
    const NativeWebSocket = window.WebSocket;
    window.WebSocket = class extends NativeWebSocket {
      constructor(url: string | URL, protocols?: string | string[]) {
        if (String(url).includes("/api/websocket/")) {
          url = String(url).replace("/api/websocket/", "/api/no-websocket/");
        }
        super(url, protocols);
      }
    };
  });
  await loginThroughPds(page);
  await page.goto(`${url}/dashboard`);
  const health = page.getByTestId("creator-health");
  await expect(health.getByRole("button", { name: /^Bitrate/ })).toContainText(
    "—",
  );
  await expect(
    health.getByText("Waiting for readings", { exact: true }),
  ).toBeVisible();
  await expect(health.locator(".recharts-cartesian-axis")).toHaveCount(0);
});

test("health charts retain samples across ranges and show gaps while disconnected", async ({
  page,
}) => {
  await page.addInitScript(() => {
    const NativeWebSocket = window.WebSocket;
    const state = window as unknown as {
      creatorSockets: WebSocket[];
      creatorDisconnected: boolean;
    };
    state.creatorSockets = [];
    state.creatorDisconnected = false;
    window.WebSocket = class extends NativeWebSocket {
      constructor(url: string | URL, protocols?: string | string[]) {
        const live = String(url).includes("/api/websocket/");
        if (live && state.creatorDisconnected) {
          url = String(url).replace("/api/websocket/", "/api/no-websocket/");
        }
        super(url, protocols);
        if (live) state.creatorSockets.push(this);
      }
    };
  });
  await loginThroughPds(page);
  await page.goto(`${url}/dashboard`);
  const health = page.getByTestId("creator-health");
  await expect(health).toBeVisible();
  await expect(
    health.getByRole("button", { name: "2m", exact: true }),
  ).toHaveAttribute("aria-pressed", "true");
  const rangeBox = await health
    .getByRole("button", { name: "2m", exact: true })
    .boundingBox();
  expect(rangeBox).not.toBeNull();
  expect(rangeBox!.height).toBeLessThanOrEqual(64);
  const bitrate = health.getByRole("button", { name: /^Bitrate/ });
  await expect(bitrate).toHaveAttribute("aria-pressed", "true");
  await expect(bitrate).toContainText("Mbps");
  await expect(health.getByText(/^Resolution /)).toContainText(/\d+x\d+/);
  const line = health.locator(".recharts-line-curve").first();
  await expect(line).toHaveAttribute("d", /[LC]/, { timeout: 20_000 });

  await health.getByRole("button", { name: "25m", exact: true }).click();
  await expect(line).toHaveAttribute("d", /[LC]/);
  await health.getByRole("button", { name: "1m", exact: true }).click();
  await expect(line).toHaveAttribute("d", /[LC]/);
  await health.getByRole("button", { name: /^Segment Δ/ }).click();
  await health.getByRole("button", { name: /^FPS/ }).click();
  await expect(health.getByText("kbps", { exact: true })).toBeVisible();
  await expect(health.getByText("ms", { exact: true })).toBeVisible();
  await expect(health.getByText("fps", { exact: true })).toBeVisible();
  await health.getByRole("button", { name: /^Viewers/ }).click();
  await expect(
    health.getByRole("button", { name: /^Viewers/ }),
  ).toHaveAttribute("aria-pressed", "true");
  await health.getByRole("button", { name: /^Chat rate/ }).click();
  await expect(health.getByText("viewers", { exact: true })).toBeVisible();
  await expect(health.getByText("msg/min", { exact: true })).toBeVisible();
  await page.setViewportSize({ width: 390, height: 844 });
  for (const control of [
    bitrate,
    health.getByRole("button", { name: "1m", exact: true }),
  ]) {
    const box = await control.boundingBox();
    expect(box).not.toBeNull();
    expect(box!.height).toBeGreaterThanOrEqual(44);
  }

  await page.evaluate(() => {
    const state = window as unknown as {
      creatorSockets: WebSocket[];
      creatorDisconnected: boolean;
    };
    state.creatorDisconnected = true;
    state.creatorSockets.forEach((socket) => socket.close());
  });
  await expect(page.getByTestId("creator-stream-status")).toHaveText(
    "Reconnecting",
  );
  await expect(bitrate).toContainText("—", { timeout: 15_000 });
  await expect(
    health.getByText("Readings stale", { exact: true }),
  ).toBeVisible();
  await expect(page.getByText("— msg/min", { exact: true })).toBeVisible();
  await page.evaluate(() => {
    (
      window as unknown as { creatorDisconnected: boolean }
    ).creatorDisconnected = false;
  });
  await expect(page.getByTestId("creator-stream-status")).toHaveText("Live");
  await expect(bitrate).toContainText("Mbps", { timeout: 15_000 });
});
