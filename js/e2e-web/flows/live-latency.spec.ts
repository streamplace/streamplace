import { expect, test } from "@playwright/test";
import { writeFile } from "node:fs/promises";
import { makeLatencyMarker } from "./live-latency-marker";

// The WHIP Authorization header is a temporary private stream key. Neither
// request traces nor evaluation arguments containing it belong in artifacts.
test.use({ trace: "off", video: "off" });
test.skip(process.env.E2E_LATENCY !== "1", "opt-in livestream measurement");

test("live latency: canvas generation to browser presentation", async ({
  page,
  browser,
}, testInfo) => {
  test.setTimeout(150_000);
  const serverUrl = process.env.SERVER_URL!;
  const streamer = process.env.ACCOUNT_DID!;
  const streamKey = process.env.E2E_STREAM_KEY!;
  expect(Boolean(serverUrl && streamer && streamKey)).toBe(true);
  const sourceWarmupMs = Number(process.env.E2E_LATENCY_SOURCE_WARMUP_MS ?? 0);
  expect(Number.isFinite(sourceWarmupMs)).toBe(true);
  expect(sourceWarmupMs).toBeGreaterThanOrEqual(0);
  expect(sourceWarmupMs).toBeLessThanOrEqual(30_000);

  // External-source mode must start with no previously ingested media. A
  // second publisher would mix frame timelines and invalidate the comparison.
  const idleUntil = Date.now() + 3000;
  do {
    const response = await page.request.get(
      `${serverUrl}/xrpc/place.stream.live.getSegments`,
      { params: { userDID: streamer, limit: 1 }, timeout: 3000 },
    );
    expect(response.ok()).toBe(true);
    expect((await response.json()).segments ?? []).toHaveLength(0);
    await page.waitForTimeout(250);
  } while (Date.now() < idleUntil);

  await page.setViewportSize({ width: 1360, height: 760 });
  await page.goto(`${serverUrl}/api/healthz`);
  await page.setContent(
    '<!doctype html><title>Live latency measurement</title><body style="display:flex;gap:16px;margin:8px"></body>',
  );
  await page.addScriptTag({
    content: `window.liveLatencyMarker = (${makeLatencyMarker.toString()})();`,
  });

  const result = await page.evaluate(measureLiveLatency, {
    serverUrl,
    streamer,
    streamKey,
    sourceWarmupMs,
  });

  const steady = result.frames.filter(
    (frame) =>
      result.steadyStart !== null &&
      result.steadyEnd !== null &&
      frame.expectedDisplayTime >= result.steadyStart &&
      frame.expectedDisplayTime < result.steadyEnd,
  );
  const valid = steady.filter(
    (frame) => frame.latencyMs !== null && frame.latencyMs >= 0,
  );
  const sorted = valid.map((frame) => frame.latencyMs!).sort((a, b) => a - b);
  const percentile = (p: number) =>
    sorted.length ? sorted[Math.floor((sorted.length - 1) * p)] : null;
  const missedCallbacks = steady
    .slice(1)
    .reduce(
      (count, frame, index) =>
        count +
        Math.max(0, frame.presentedFrames - steady[index].presentedFrames - 1),
      0,
    );
  const times = result.frames.map((frame) => ({
    at: frame.expectedDisplayTime,
    observed: true,
  }));
  if (result.steadyStart !== null && result.steadyEnd !== null) {
    if (!times.some((time) => time.at <= result.steadyStart!))
      times.unshift({ at: result.steadyStart, observed: false });
    if (!times.some((time) => time.at >= result.steadyEnd!))
      times.push({ at: result.steadyEnd, observed: false });
  }
  const displayGaps = times
    .slice(1)
    .map((time, index) => ({
      from: times[index].at,
      to: time.at,
      displayGapMs: time.at - times[index].at,
      windowOverlapMs: Math.max(
        0,
        Math.min(time.at, result.steadyEnd ?? 0) -
          Math.max(times[index].at, result.steadyStart ?? 0),
      ),
      // Without a surrounding callback, a window boundary gives a lower bound
      // on the freeze rather than a complete presentation-to-presentation gap.
      censored: !time.observed || !times[index].observed,
    }))
    .filter((gap) => gap.windowOverlapMs > 0);
  const ids = new Set(valid.map((frame) => frame.decodedId!));
  const firstId = ids.size ? Math.min(...ids) : null;
  const lastId = ids.size ? Math.max(...ids) : null;
  const offered = result.source.offeredFrames.filter(
    (frame) =>
      frame.generatedAt >= (result.steadyStart ?? Infinity) &&
      frame.generatedAt < (result.steadyEnd ?? 0),
  );
  const calibrated = result.calibration.frames.filter(
    (frame) => frame.latencyMs !== null && frame.latencyMs >= 0,
  );
  const summary = {
    validFrames: valid.length,
    uniqueSourceFrames: ids.size,
    invalidMarkers: steady.length - valid.length,
    duplicateIds: valid.length - ids.size,
    missedCallbacks,
    medianMs: sorted.length
      ? (sorted[Math.floor((sorted.length - 1) / 2)] +
          sorted[Math.floor(sorted.length / 2)]) /
        2
      : null,
    p95Ms: percentile(0.95),
    calibration: {
      validFrames: calibrated.length,
      invalidMarkers: result.calibration.frames.length - calibrated.length,
    },
    observedSourceSpan: {
      firstId,
      lastId,
      missingIds:
        firstId === null || lastId === null
          ? null
          : lastId - firstId + 1 - ids.size,
    },
    // Generation and presentation windows describe different source cohorts;
    // their counts are cadence evidence and must not be divided into a loss rate.
    offeredCanvasFrames: offered.length,
    offeredFps: offered.length / 30,
    observedUniqueFps: ids.size / 30,
    stalls: displayGaps.filter((gap) => gap.displayGapMs > 250),
  };
  const artifact = {
    browserVersion: browser.version(),
    ...result,
    summary,
    completion: [] as object[],
  };
  const artifactPath = testInfo.outputPath("live-latency.json");
  await writeFile(artifactPath, JSON.stringify(artifact, null, 2));
  expect(result.failure).toBeNull();
  expect(
    result.startup.firstFrameFromPublishMs! -
      result.startup.firstFrameFromPlaybackMs!,
  ).toBeGreaterThanOrEqual(sourceWarmupMs);
  expect(result.received).toEqual({ width: 640, height: 360 });
  expect(summary.uniqueSourceFrames).toBeGreaterThanOrEqual(600);
  expect(
    result.calibration.frames.filter(
      (frame) => frame.latencyMs !== null && frame.latencyMs >= 0,
    ).length,
  ).toBeGreaterThanOrEqual(30);

  // Fetch catalog and audio bytes after timing, so an HLS client cannot add
  // work to the latency window. The metadata-only segment endpoint hardcodes
  // Opus and cannot establish that AAC completion really ran.
  const masterUrl = `${serverUrl}/xrpc/place.stream.playback.getLivePlaylist?streamer=${encodeURIComponent(streamer)}`;
  const master = await page.request.get(masterUrl, { timeout: 5000 });
  expect(master.ok()).toBe(true);
  const playlist = await master.text();
  for (const codec of [/^opus$/i, /^mp4a/]) {
    const line = playlist
      .split("\n")
      .find(
        (line) =>
          line.startsWith("#EXT-X-MEDIA:TYPE=AUDIO") &&
          codec.test(line.match(/NAME="([^"]+)"/)?.[1] ?? ""),
      );
    expect(line, `completed audio rendition ${codec}`).toBeDefined();
    const name = line!.match(/NAME="([^"]+)"/)![1];
    const mediaUrl = new URL(line!.match(/URI="([^"]+)"/)![1], masterUrl).href;
    const response = await page.request.get(mediaUrl, { timeout: 5000 });
    expect(response.ok()).toBe(true);
    const media = await response.text();
    const durations = Array.from(media.matchAll(/#EXTINF:([\d.]+)/g), (match) =>
      Number(match[1]),
    );
    expect(durations.length).toBeGreaterThan(0);
    const segmentUri = media
      .split("\n")
      .find((line) => line && !line.startsWith("#"))!;
    const initUri = media.match(/#EXT-X-MAP:URI="([^"]+)"/)?.[1];
    expect(initUri).toBeDefined();
    const bytes: number[] = [];
    for (const uri of [initUri!, segmentUri]) {
      const segment = await page.request.get(new URL(uri, mediaUrl).href, {
        timeout: 5000,
      });
      expect(segment.ok()).toBe(true);
      bytes.push((await segment.body()).byteLength);
      expect(bytes.at(-1)).toBeGreaterThan(0);
    }
    artifact.completion.push({
      codec: name,
      durations,
      initBytes: bytes[0],
      segmentBytes: bytes[1],
    });
  }
  await writeFile(artifactPath, JSON.stringify(artifact, null, 2));
  const reports = result.rtc.after!;
  for (const [side, kind, expectedCodec] of [
    ["publisher", "video", "video/h264"],
    ["publisher", "audio", "audio/opus"],
    ["viewer", "video", "video/h264"],
    ["viewer", "audio", "audio/opus"],
  ] as const) {
    const rtp = reports[side].find(
      (report) =>
        report.type ===
          (side === "publisher" ? "outbound-rtp" : "inbound-rtp") &&
        report.kind === kind,
    );
    expect(rtp).toBeDefined();
    const codec = reports[side].find((report) => report.id === rtp!.codecId);
    expect(String(codec?.mimeType).toLowerCase()).toBe(expectedCodec);
    expect(
      Number(rtp![side === "publisher" ? "packetsSent" : "packetsReceived"]),
    ).toBeGreaterThan(0);
    const earlier = result.rtc.before![side].find(
      (report) => report.id === rtp!.id,
    );
    expect(earlier).toBeDefined();
    const byteField = side === "publisher" ? "bytesSent" : "bytesReceived";
    expect(
      Number(rtp![byteField]) - Number(earlier![byteField]),
    ).toBeGreaterThan(0);
  }
  await testInfo.attach("live-latency", {
    path: artifactPath,
    contentType: "application/json",
  });
  console.log(
    `live latency: ${JSON.stringify({ ...summary, stalls: summary.stalls.length })}`,
  );
});

// Runs in the browser via page.evaluate; Playwright serializes only this
// function, so every helper and piece of runtime state it needs stays nested
// inside it. The marker comes from window, injected by the spec beforehand.
async function measureLiveLatency({
  serverUrl,
  streamer,
  streamKey,
  sourceWarmupMs,
}: {
  serverUrl: string;
  streamer: string;
  streamKey: string;
  sourceWarmupMs: number;
}) {
  const marker = (
    window as typeof window & {
      liveLatencyMarker: ReturnType<typeof makeLatencyMarker>;
    }
  ).liveLatencyMarker;
  type Frame = {
    decodedId: number | null;
    generatedAt: number | null;
    latencyMs: number | null;
    callbackNow: number;
    expectedDisplayTime: number;
    presentationTime: number;
    presentedFrames: number;
    mediaTime: number;
  };
  const delay = (ms: number) => new Promise((r) => setTimeout(r, ms));
  async function waitFor(check: () => boolean, name: string, ms: number) {
    const end = performance.now() + ms;
    while (!check()) {
      if (performance.now() >= end) throw new Error(`timed out: ${name}`);
      await delay(20);
    }
  }
  function videoElement() {
    const video = document.createElement("video");
    video.autoplay = true;
    video.muted = true;
    video.playsInline = true;
    video.width = 640;
    video.height = 360;
    document.body.append(video);
    return video;
  }
  function source() {
    const canvas = document.createElement("canvas");
    canvas.width = 640;
    canvas.height = 360;
    document.body.append(canvas);
    const context = canvas.getContext("2d")!;
    const stream = canvas.captureStream(0);
    const track = stream.getVideoTracks()[0] as CanvasCaptureMediaStreamTrack;
    // The test deadline bounds this map; retaining IDs lets late frames
    // match their actual generation time rather than an inferred cadence.
    const generated = new Map<number, number>();
    let id = 0;
    const draw = () => {
      const now = performance.now();
      generated.set(id, now);
      context.fillStyle = "#204060";
      context.fillRect(0, 0, 640, 360);
      for (let square = 0; square < 12; square++) {
        context.fillStyle = `hsl(${square * 30},80%,55%)`;
        context.fillRect(
          (now / 5 + square * 53) % 640,
          170 + ((id + square * 19) % 140),
          44,
          44,
        );
      }
      marker.encode(id).forEach((bit, index) => {
        context.fillStyle = bit ? "white" : "black";
        context.fillRect(
          marker.x + (index % marker.columns) * marker.cellSize,
          marker.y + Math.floor(index / marker.columns) * marker.cellSize,
          marker.cellSize,
          marker.cellSize,
        );
      });
      track.requestFrame();
      id++;
    };
    draw();
    const timer = setInterval(draw, 1000 / 30);
    return {
      stream,
      generated,
      stop() {
        clearInterval(timer);
        stream.getTracks().forEach((track) => track.stop());
        canvas.remove();
      },
    };
  }
  function observe(video: HTMLVideoElement, generated: Map<number, number>) {
    const reader = document.createElement("canvas");
    reader.width = marker.columns;
    reader.height = marker.bits / marker.columns;
    const context = reader.getContext("2d", { willReadFrequently: true })!;
    context.imageSmoothingEnabled = false;
    const frames: Frame[] = [];
    let handle = 0;
    const read = (now: number, metadata: VideoFrameCallbackMetadata) => {
      context.drawImage(
        video,
        marker.x,
        marker.y,
        marker.columns * marker.cellSize,
        reader.height * marker.cellSize,
        0,
        0,
        reader.width,
        reader.height,
      );
      const pixels = context.getImageData(
        0,
        0,
        reader.width,
        reader.height,
      ).data;
      const brightness = Array.from(
        { length: marker.bits },
        (_, index) =>
          (pixels[index * 4] + pixels[index * 4 + 1] + pixels[index * 4 + 2]) /
          3,
      );
      const decodedId = marker.decode(brightness);
      const generatedAt =
        decodedId === null ? null : (generated.get(decodedId) ?? null);
      frames.push({
        decodedId,
        generatedAt,
        latencyMs:
          generatedAt === null
            ? null
            : metadata.expectedDisplayTime - generatedAt,
        callbackNow: now,
        expectedDisplayTime: metadata.expectedDisplayTime,
        presentationTime: metadata.presentationTime,
        presentedFrames: metadata.presentedFrames,
        mediaTime: metadata.mediaTime,
      });
      handle = video.requestVideoFrameCallback(read);
    };
    handle = video.requestVideoFrameCallback(read);
    return { frames, stop: () => video.cancelVideoFrameCallback(handle) };
  }
  async function exchange(
    name: "WHIP" | "WHEP",
    pc: RTCPeerConnection,
    endpoint: string,
    key?: string,
  ) {
    const negotiation = {
      name,
      offer: [] as ReturnType<typeof mediaDirections>,
      answer: [] as ReturnType<typeof mediaDirections>,
      httpStatus: null as number | null,
    };
    negotiations.push(negotiation);
    try {
      const offer = await pc.createOffer();
      const opus = offer.sdp!.match(/^a=rtpmap:(\d+) opus\//im)?.[1];
      if (!opus) throw new Error("offer has no Opus codec");
      const fmtp = new RegExp(`^a=fmtp:${opus} ([^\\r\\n]*)`, "m");
      if (!fmtp.test(offer.sdp!))
        throw new Error("offer has no Opus parameters");
      offer.sdp = offer.sdp!.replace(
        fmtp,
        (_, parameters: string) =>
          `a=fmtp:${opus} ${parameters
            .split(";")
            .filter((parameter) => !parameter.startsWith("stereo="))
            .concat("stereo=1")
            .join(";")}`,
      );
      await pc.setLocalDescription(offer);
      await waitFor(
        () => pc.iceGatheringState === "complete",
        "browser ICE gathering",
        5000,
      );
      negotiation.offer = mediaDirections(pc.localDescription!.sdp);
      const response = await fetch(endpoint, {
        method: "POST",
        headers: {
          "content-type": "application/sdp",
          ...(key ? { Authorization: `Bearer ${key}` } : {}),
        },
        body: pc.localDescription!.sdp,
        signal: AbortSignal.timeout(30_000),
      });
      negotiation.httpStatus = response.status;
      if (!response.ok)
        throw new Error(
          `SDP exchange ${response.status}: ${await response.text()}`,
        );
      const answer = await response.text();
      negotiation.answer = mediaDirections(answer);
      await pc.setRemoteDescription({
        type: "answer",
        sdp: answer,
      });
    } catch (error) {
      throw new Error(`${name}: ${String(error)}`);
    }
  }
  // Keep negotiation diagnostics useful without recording ICE credentials,
  // fingerprints, stream keys, or complete SDP payloads.
  function mediaDirections(sdp: string) {
    return sdp
      .split(/^m=/m)
      .slice(1)
      .map((section) => ({
        kind: section.match(/^(audio|video|application)\b/)?.[1] ?? "other",
        mid: section.match(/^a=mid:([^\r\n]+)/m)?.[1] ?? null,
        direction:
          section.match(/^a=(sendrecv|sendonly|recvonly|inactive)\r?$/m)?.[1] ??
          "sendrecv",
      }));
  }
  const negotiations: {
    name: string;
    offer: ReturnType<typeof mediaDirections>;
    answer: ReturnType<typeof mediaDirections>;
    httpStatus: number | null;
  }[] = [];
  async function stats(pc: RTCPeerConnection) {
    const reports: Record<string, unknown>[] = [];
    (await pc.getStats()).forEach((report) => reports.push({ ...report }));
    return reports;
  }

  // Calibration tests the same painted cells and timestamp lookup through
  // a real local MediaStream before any node latency is measured.
  const calibrationSource = source();
  const calibrationVideo = videoElement();
  const calibration = observe(calibrationVideo, calibrationSource.generated);
  let calibrationFailure: string | null = null;
  try {
    calibrationVideo.srcObject = calibrationSource.stream;
    await calibrationVideo.play();
    await waitFor(
      () =>
        calibration.frames.filter((f) => f.generatedAt !== null).length >= 30,
      "direct canvas calibration",
      5000,
    );
  } catch (error) {
    calibrationFailure = String(error);
  } finally {
    calibration.stop();
    calibrationSource.stop();
    calibrationVideo.remove();
  }
  const input = source();
  const video = videoElement();
  const remote = observe(video, input.generated);
  const publisher = new RTCPeerConnection({ bundlePolicy: "max-bundle" });
  const viewer = new RTCPeerConnection({ bundlePolicy: "max-bundle" });
  const audioContext = new AudioContext({ sampleRate: 48000 });
  const oscillator = audioContext.createOscillator();
  const destination = audioContext.createMediaStreamDestination();
  oscillator.connect(destination);
  oscillator.start();
  const start = performance.now();
  let playbackStart: number | null = null;
  let steadyStart: number | null = null;
  let steadyEnd: number | null = null;
  let received = { width: 0, height: 0 };
  let failure: string | null = calibrationFailure;
  type PeerStats = {
    publisher: Record<string, unknown>[];
    viewer: Record<string, unknown>[];
  };
  let before: PeerStats | null = null;
  let after: PeerStats | null = null;
  try {
    if (calibrationFailure) throw new Error(calibrationFailure);
    await audioContext.resume();
    for (const [kind, track] of [
      ["audio", destination.stream.getAudioTracks()[0]],
      ["video", input.stream.getVideoTracks()[0]],
    ] as const) {
      // Match the working CLI publisher's audio-first AddTrack profile.
      // The node currently answers sendrecv for its ingest transceivers.
      const transceiver = publisher.addTransceiver(track, {
        direction: "sendrecv",
      });
      const codecs = RTCRtpSender.getCapabilities(kind)!.codecs.filter(
        (codec) =>
          codec.mimeType.toLowerCase() ===
          (kind === "video" ? "video/h264" : "audio/opus"),
      );
      if (!codecs.length)
        throw new Error(
          `browser cannot send ${kind === "video" ? "H264" : "Opus"}`,
        );
      transceiver.setCodecPreferences(codecs);
    }
    await exchange(
      "WHIP",
      publisher,
      `${serverUrl}/api/ingest/webrtc`,
      streamKey,
    );
    await waitFor(
      () => publisher.connectionState === "connected",
      "WHIP connection",
      15_000,
    );
    if (sourceWarmupMs > 0) await delay(sourceWarmupMs);
    viewer.addTransceiver("video", { direction: "recvonly" });
    viewer.addTransceiver("audio", { direction: "recvonly" });
    viewer.ontrack = (event) => {
      video.srcObject = event.streams[0];
    };
    playbackStart = performance.now();
    const endpoint = new URL(`${serverUrl}/xrpc/place.stream.playback.whep`);
    endpoint.searchParams.set("streamer", streamer);
    endpoint.searchParams.set("rendition", "source");
    await exchange("WHEP", viewer, endpoint.href);
    await video.play();
    await waitFor(
      () => remote.frames.some((frame) => frame.generatedAt !== null),
      "first decoded remote marker",
      30_000,
    );
    await delay(10_000);
    before = {
      publisher: await stats(publisher),
      viewer: await stats(viewer),
    };
    steadyStart = performance.now();
    steadyEnd = steadyStart + 30_000;
    // Keep callbacks alive one extra frame so a callback delivered late
    // can still describe a presentation inside the fixed window.
    await delay(30_100);
    after = {
      publisher: await stats(publisher),
      viewer: await stats(viewer),
    };
    received = { width: video.videoWidth, height: video.videoHeight };
  } catch (error) {
    failure = String(error);
  } finally {
    remote.stop();
    publisher.close();
    viewer.close();
    input.stop();
    destination.stream.getTracks().forEach((track) => track.stop());
    oscillator.stop();
    await audioContext.close();
  }
  const first = remote.frames.find((frame) => frame.generatedAt !== null);
  return {
    failure,
    calibration: {
      failure: calibrationFailure,
      frames: calibration.frames,
    },
    userAgent: navigator.userAgent,
    source: {
      width: 640,
      height: 360,
      requestedFps: 30,
      offeredFrames: Array.from(input.generated, ([id, generatedAt]) => ({
        id,
        generatedAt,
      })),
    },
    received,
    startup: {
      firstFrameFromPublishMs: first ? first.expectedDisplayTime - start : null,
      firstFrameFromPlaybackMs:
        first && playbackStart !== null
          ? first.expectedDisplayTime - playbackStart
          : null,
    },
    warmupMs: 10_000,
    sourceWarmupMs,
    steadyStart,
    steadyEnd,
    frames: remote.frames,
    negotiations,
    rtc: { before, after },
  };
}
