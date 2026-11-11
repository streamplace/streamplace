import {
  expect,
  test,
  type Browser,
  type BrowserContext,
} from "@playwright/test";
import { mkdirSync, readFileSync, writeFileSync } from "node:fs";
import { join, resolve } from "node:path";
import { uploadFixture, watchPath } from "../flows/vod-upload";
import { pointAppAtNode } from "../server-setup";

// VOD time to first frame: ms from navigation start to the first presented
// video frame, arriving at /<handle>/video/<tid> cold (fresh browser, empty
// HTTP cache) and warm (same browser, just after watching another video).
const HTTPS_URL = process.env.SERVER_HTTPS_URL!;
const HANDLE = process.env.ACCOUNT_HANDLE!;
const RUNS = Number(process.env.VOD_TTFF_RUNS || 5);
const RESULTS_DIR = join(__dirname, "..", "bench-results");

// Any node: two comma-separated watch URLs (primer, then target) replace the
// harness's uploads. VOD_TTFF_BASELINE may name a previous run's vod-ttff.json.
const REMOTE = process.env.VOD_TTFF_VIDEOS?.split(",");
// Relative to js/e2e-web.
const BASELINE_FILE =
  process.env.VOD_TTFF_BASELINE ??
  (REMOTE ? undefined : "bench/vod-ttff-baseline.json");
// A scenario regresses past both of these over its baseline.
const TOLERANCE = { ratio: 0.25, ms: 300 };

// Chromium network emulation per network. Without throttling, on loopback,
// bundle and segment bytes are nearly free.
const NETWORKS = {
  unthrottled: undefined,
  throttled: {
    latency: 40,
    downloadThroughput: 10_000_000 / 8,
    uploadThroughput: 5_000_000 / 8,
  },
};
const STARTS = ["cold", "warm"] as const;

type Baseline = { firstFrameMs: Record<string, number> };

// Milestones in ms since navigation start; null when the page never made
// that request. Requests count from when the app made them: Chromium's
// network emulation delays responses past their resource timing.
type Milestones = {
  domContentLoaded: number;
  videoMounted: number;
  playlistRequested: number | null;
  blobRequested: number | null;
  firstFrame: number;
  transferKB: number;
};
const COLUMNS: [keyof Milestones, string][] = [
  ["domContentLoaded", "DOMContentLoaded"],
  ["videoMounted", "`<video>` mounted"],
  ["playlistRequested", "Playlist requested"],
  ["blobRequested", "First blob requested"],
  ["firstFrame", "First frame"],
  ["transferKB", "Fetched by first frame (KB)"],
];

type Marks = { videoMounted?: number; firstFrame?: number };
declare global {
  interface Window {
    __vodTtff: Marks;
  }
}

// Runs before the app: timestamps the first <video> and its first frame.
function instrument() {
  performance.setResourceTimingBufferSize(10_000);
  const marks: Marks = (window.__vodTtff = {});
  const videos = document.getElementsByTagName("video");
  const seen = new WeakSet<HTMLVideoElement>();
  new MutationObserver(() => {
    for (const video of videos) {
      if (seen.has(video)) continue;
      seen.add(video);
      marks.videoMounted ??= performance.now();
      video.requestVideoFrameCallback((_now, frame) => {
        marks.firstFrame ??= frame.presentationTime;
      });
    }
  }).observe(document, { childList: true, subtree: true });
}

function collect(): Milestones {
  const marks = window.__vodTtff as Required<Marks>;
  const [nav] = performance.getEntriesByType(
    "navigation",
  ) as PerformanceNavigationTiming[];
  const resources = performance.getEntriesByType(
    "resource",
  ) as PerformanceResourceTiming[];
  const requested = (method: string) =>
    resources.find((r) => r.name.includes(`/xrpc/${method}`))?.startTime ??
    null;
  const bytes = [nav, ...resources]
    .filter((r) => r.startTime < marks.firstFrame)
    .reduce((sum, r) => sum + r.transferSize, 0);
  return {
    domContentLoaded: nav.domContentLoadedEventEnd,
    videoMounted: marks.videoMounted,
    playlistRequested: requested("place.stream.playback.getVideoPlaylist"),
    blobRequested: requested("place.stream.playback.getVideoBlob"),
    firstFrame: marks.firstFrame,
    transferKB: bytes / 1024,
  };
}

// Open `url` in a new page of `context` and wait for its first frame.
async function open(
  context: BrowserContext,
  url: string,
  network: (typeof NETWORKS)[keyof typeof NETWORKS],
) {
  const page = await context.newPage();
  if (network) {
    const cdp = await context.newCDPSession(page);
    await cdp.send("Network.emulateNetworkConditions", {
      offline: false,
      ...network,
    });
  }
  await page.goto(url);
  await page.waitForFunction(() => window.__vodTtff.firstFrame !== undefined);
  return page;
}

function median(values: number[]): number {
  const sorted = [...values].sort((a, b) => a - b);
  const mid = sorted.length / 2;
  return Number.isInteger(mid)
    ? (sorted[mid - 1] + sorted[mid]) / 2
    : sorted[Math.floor(mid)];
}

// Upload two harness videos and point the app at the harness node. Each upload
// logs in from its own context; the measured contexts stay logged out.
async function harnessVideos(browser: Browser) {
  const videos = await Promise.all(
    [0, 1].map(async () => {
      const context = await browser.newContext();
      try {
        const uri = await uploadFixture(await context.newPage());
        return `${HTTPS_URL}${watchPath(uri, HANDLE)}`;
      } finally {
        await context.close();
      }
    }),
  );
  // The app's node setting, without the HTTP cache that setting it warmed.
  const setup = await browser.newContext();
  await pointAppAtNode(await setup.newPage(), HTTPS_URL);
  const storageState = await setup.storageState();
  await setup.close();
  return { videos, storageState };
}

test.skip(
  !process.env.SERVER_HTTPS_URL && !REMOTE,
  "harness started without HTTPS",
);

test("vod-ttff: time to first frame, cold and warm", async ({ browser }) => {
  test.setTimeout(30 * 60_000);
  const baseline: Baseline | undefined =
    BASELINE_FILE &&
    JSON.parse(readFileSync(resolve(__dirname, "..", BASELINE_FILE), "utf8"));

  // Two videos, so the warm run's video is one this browser has not seen.
  // Production builds of the app already talk to the node that serves them.
  if (REMOTE) {
    expect(REMOTE, "VOD_TTFF_VIDEOS holds two watch URLs").toHaveLength(2);
  }
  const {
    videos: [primer, target],
    storageState,
  } = REMOTE
    ? { videos: REMOTE, storageState: undefined }
    : await harnessVideos(browser);

  const runs: Record<string, Milestones[]> = {};
  for (const [network, conditions] of Object.entries(NETWORKS)) {
    for (let i = 0; i < RUNS; i++) {
      for (const start of STARTS) {
        const context = await browser.newContext({ storageState });
        await context.addInitScript(instrument);
        if (start === "warm") {
          // "Watched" means it played a little, not just painted a frame.
          const page = await open(context, primer, conditions);
          await page.waitForFunction(
            () => document.querySelector("video")!.currentTime >= 3,
          );
          await page.close();
        }
        const page = await open(context, target, conditions);
        (runs[`${network}/${start}`] ??= []).push(await page.evaluate(collect));
        await context.close();
      }
    }
  }

  const firstFrameMs = Object.fromEntries(
    Object.entries(runs).map(([scenario, results]) => [
      scenario,
      Math.round(median(results.map((r) => r.firstFrame))),
    ]),
  );
  const { ratio, ms } = TOLERANCE;
  const limit = (base: number) =>
    Math.round(Math.max(base * (1 + ratio), base + ms));

  const rows = Object.entries(runs).map(([scenario, results]) => {
    const value = firstFrameMs[scenario];
    const base = baseline?.firstFrameMs[scenario];
    const frames = results.map((r) => r.firstFrame);
    const status =
      base === undefined
        ? "no baseline"
        : value > limit(base)
          ? "**regressed**"
          : value < base / (1 + ratio)
            ? "faster: update the baseline?"
            : "ok";
    return `| ${scenario} | ${value} | ${Math.round(Math.min(...frames))}–${Math.round(Math.max(...frames))} | ${base ?? "–"} | ${base === undefined ? "–" : limit(base)} | ${status} |`;
  });
  const breakdown = Object.entries(runs).map(([scenario, results]) => {
    const cells = COLUMNS.map(([key]) => {
      const values = results
        .map((r) => r[key])
        .filter((v): v is number => v !== null);
      return values.length ? Math.round(median(values)) : "–";
    });
    return `| ${scenario} | ${cells.join(" | ")} |`;
  });
  const report = [
    `### VOD time to first frame: ${new URL(target).origin}`,
    "",
    `Median of ${RUNS} runs, ms from navigation start. ${BASELINE_FILE ? `Baseline: \`${BASELINE_FILE}\`; a scenario regresses past both +${ratio * 100}% and +${ms} ms.` : "No baseline."}`,
    "",
    "| Scenario | First frame | Range | Baseline | Limit | |",
    "|---|---|---|---|---|---|",
    ...rows,
    "",
    "<details><summary>Median milestones (ms)</summary>",
    "",
    `| Scenario | ${COLUMNS.map(([, title]) => title).join(" | ")} |`,
    `|---|${COLUMNS.map(() => "---|").join("")}`,
    ...breakdown,
    "",
    "</details>",
    "",
  ].join("\n");

  mkdirSync(RESULTS_DIR, { recursive: true });
  writeFileSync(join(RESULTS_DIR, "vod-ttff.md"), report);
  writeFileSync(
    join(RESULTS_DIR, "vod-ttff.json"),
    JSON.stringify({ firstFrameMs, runs }, null, 2),
  );
  console.log(report);

  if (!baseline) return;
  for (const [scenario, value] of Object.entries(firstFrameMs)) {
    const base = baseline.firstFrameMs[scenario];
    expect.soft(base, `${scenario} has a baseline`).toBeDefined();
    if (base !== undefined) {
      expect
        .soft(value, `${scenario} median first frame (baseline ${base} ms)`)
        .toBeLessThanOrEqual(limit(base));
    }
  }
});
