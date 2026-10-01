import { mkdirSync, readFileSync } from "node:fs";
import { createRequire } from "node:module";
import { resolve } from "node:path";
const require = createRequire(resolve("js/e2e-web/package.json"));
const { chromium } = require("playwright");
const base = process.argv[2];
const fixture = readFileSync(process.argv[3]);
let offset = 12;
let rate = 0;
let channels = 0;
let bits = 0;
let pcm;
while (offset + 8 <= fixture.length) {
  const id = fixture.toString("ascii", offset, offset + 4);
  const size = fixture.readUInt32LE(offset + 4);
  if (id === "fmt ") {
    if (fixture.readUInt16LE(offset + 8) !== 1)
      throw new Error("Fixture must be PCM WAV");
    channels = fixture.readUInt16LE(offset + 10);
    rate = fixture.readUInt32LE(offset + 12);
    bits = fixture.readUInt16LE(offset + 22);
  }
  if (id === "data") {
    if (rate !== 16000 || channels !== 1 || bits !== 16)
      throw new Error("Fixture must be 16 kHz mono int16 WAV");
    pcm = Array.from(
      { length: size / 2 },
      (_, index) => fixture.readInt16LE(offset + 8 + index * 2) / 32768,
    );
    break;
  }
  offset += 8 + size + (size % 2);
}
if (!pcm) throw new Error("Fixture has no audio");
mkdirSync(".build/captionobs-smoke", { recursive: true });
const browser = await chromium.launch({
  headless: true,
  args: ["--disable-dev-shm-usage", "--no-sandbox"],
});
try {
  const page = await browser.newPage();
  page.on("pageerror", (error) => console.error("PAGE ERROR", error.message));
  await page.goto(`${base}/captioner`, { waitUntil: "domcontentloaded" });
  console.log(
    "isolation",
    await page.evaluate(() => ({
      isolated: crossOriginIsolated,
      sharedArrayBuffer: Boolean(globalThis.SharedArrayBuffer),
    })),
  );
  for (const model of ["tiny", "base", "small"]) {
    const result = await page.evaluate(
      async ({ base, model, pcm }) => {
        const worker = new Worker(
          `${base}/api/captioner/4979e04f5dcaccb36057e059bbaed8a2f5288315/worker.js`,
          { type: "module" },
        );
        try {
          return await new Promise((resolve, reject) => {
            worker.onerror = (event) => reject(new Error(event.message));
            worker.onmessage = (event) => {
              if (event.data.type === "ready") resolve(event.data);
              if (event.data.type === "error")
                reject(new Error(event.data.message));
            };
            const audio = new Float32Array(pcm);
            worker.postMessage(
              {
                type: "init",
                model,
                language: "en",
                threads: 4,
                benchmarkPCM: audio,
              },
              [audio.buffer],
            );
          });
        } finally {
          worker.terminate();
        }
      },
      { base, model, pcm },
    );
    const text = result.benchmark.segments
      .map((segment) => segment.text)
      .join(" ");
    if (!text.toLowerCase().includes("ask not"))
      throw new Error(`${model} failed voiced inference: ${text}`);
    console.log(
      JSON.stringify({
        model,
        audioSeconds: pcm.length / 16000,
        threads: 4,
        rtf: result.rtf,
        language: result.benchmark.language,
        text,
      }),
    );
  }
  await page.screenshot({
    path: ".build/captionobs-smoke/captioner.png",
    fullPage: true,
  });
  console.log("captioner heading", await page.locator("h1").allTextContents());
  let overlaySocket;
  const cue = {
    $type: "place.stream.caption.defs#liveCue",
    id: "smoke-cue",
    track: {
      id: "ingest-en",
      language: "en",
      source: "ingest",
      origin: "canonical",
    },
    startTime: new Date().toISOString(),
    endTime: new Date().toISOString(),
    text: "Interim caption",
    final: false,
  };
  await page.routeWebSocket("**/api/websocket/**", (socket) => {
    overlaySocket = socket;
    socket.send(JSON.stringify(cue));
  });
  await page.goto(
    `${base}/embed/captions/did:plc:captionobssmoke?fontSize=42&maxLines=3&color=white&background=black&position=top`,
  );
  await page.getByText("Interim caption", { exact: true }).waitFor();
  overlaySocket.send(
    JSON.stringify({
      ...cue,
      text: "Final caption from any source",
      final: true,
    }),
  );
  await page
    .getByText("Final caption from any source", { exact: true })
    .waitFor();
  console.log(
    "overlay simulated interim/final",
    await page.getByLabel("Live captions").innerText(),
  );
  await page.screenshot({
    path: ".build/captionobs-smoke/overlay.png",
    omitBackground: true,
  });
  console.log(
    "overlay background",
    await page.evaluate(() => getComputedStyle(document.body).backgroundColor),
  );
} finally {
  await browser.close();
}
