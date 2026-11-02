import {
  handleWebSocketMessages,
  makeLivestreamStore,
} from "@streamplace/core";
import type { place } from "streamplace";
import { beforeAll, describe, expect, it, vi } from "vitest";
import {
  appendCreatorMetric,
  creatorMetricsInRange,
  sampleCreatorMetrics,
} from "../../app/components/live-dashboard/creator-metrics";

const warnings = vi.hoisted(() =>
  vi.spyOn(console, "warn").mockImplementation(() => {}),
);
beforeAll(() => {
  for (const [message] of warnings.mock.calls) {
    expect(message).toBe(
      "[@atproto/lex-data]: Uint8Array.fromBase64 / Uint8Array.prototype.toBase64 not available in this environment. Falling back to ponyfill implementation.",
    );
  }
  warnings.mockRestore();
});

const now = Date.parse("2026-10-26T12:00:00Z");
const segment = {
  $type: "place.stream.segment",
  startTime: new Date(now - 1000).toISOString(),
  size: 250_000,
  duration: 1_000_000_000,
  video: [{ width: 1920, height: 1080, framerate: { num: 30_000, den: 1001 } }],
} as place.stream.segment.Main;

function message(id: string, at: number, did = "did:plc:viewer") {
  return {
    $type: "place.stream.chat.defs#messageView",
    uri: `at://${did}/place.stream.chat.message/${id}`,
    cid: id,
    author: { did, handle: "viewer.example" },
    indexedAt: new Date(at).toISOString(),
    record: {
      $type: "place.stream.chat.message",
      text: id,
      createdAt: new Date(at).toISOString(),
    },
  };
}

describe("creator metric samples", () => {
  it("uses actual store events, preserves units, and counts only recent viewer messages", () => {
    const store = makeLivestreamStore();
    store.setState((state) =>
      handleWebSocketMessages(state, [
        segment,
        { $type: "place.stream.livestream#viewerCount", count: 7 },
        message("recent", now - 1000),
        message("old", now - 61_000),
        message("future", now + 1000),
        message("system", now - 1000, "did:sys:system"),
      ] as any),
    );
    expect(sampleCreatorMetrics(store.getState(), now, 1200)).toEqual({
      at: now,
      bitrateKbps: 2000,
      fps: 30_000 / 1001,
      segmentMs: 1200,
      viewers: 7,
      chatRate: 1,
    });
  });

  it("uses gaps for missing or invalid readings and stops presenting stale media", () => {
    const store = makeLivestreamStore();
    expect(sampleCreatorMetrics(store.getState(), now, null)).toEqual({
      at: now,
      bitrateKbps: null,
      fps: null,
      segmentMs: null,
      viewers: null,
      chatRate: null,
    });
    store.setState((state) =>
      handleWebSocketMessages(state, [
        {
          ...segment,
          duration: 0,
          video: [{ framerate: { num: 30, den: 0 } }],
        },
      ] as any),
    );
    const invalid = sampleCreatorMetrics(store.getState(), now, NaN);
    expect(invalid).toMatchObject({
      bitrateKbps: null,
      fps: null,
      segmentMs: null,
      viewers: null,
      chatRate: 0,
    });
    store.setState((state) => handleWebSocketMessages(state, [segment]));
    expect(
      sampleCreatorMetrics(store.getState(), now + 9000, 2000),
    ).toMatchObject({ bitrateKbps: null, fps: null, segmentMs: null });
    store.setState({ websocketConnected: false, viewers: 7 });
    expect(sampleCreatorMetrics(store.getState(), now, 2000)).toMatchObject({
      bitrateKbps: null,
      fps: null,
      segmentMs: null,
      viewers: null,
      chatRate: null,
    });
  });

  it.each([0, -1, NaN, Infinity, 0.5])(
    "accepts positive timing and non-negative integer viewer counts: %s",
    (value) => {
      const store = makeLivestreamStore();
      store.setState((state) => handleWebSocketMessages(state, [segment]));
      store.setState({ viewers: value });
      const sample = sampleCreatorMetrics(store.getState(), now, value);
      expect(sample.segmentMs).toBe(value === 0.5 ? 0.5 : null);
      expect(sample.viewers).toBe(value === 0 ? 0 : null);
    },
  );
});

describe("creator metric history", () => {
  const point = (at: number): ReturnType<typeof sampleCreatorMetrics> => ({
    at,
    bitrateKbps: 1000,
    fps: 30,
    segmentMs: 1000,
    viewers: 1,
    chatRate: 0,
  });

  it("retains up to 25 minutes when changing the displayed range", () => {
    let history: ReturnType<typeof point>[] = [];
    for (let i = 0; i < 760; i++)
      history = appendCreatorMetric(history, point(i * 2000));
    expect(history).toHaveLength(750);
    const end = history[history.length - 1].at;
    expect(creatorMetricsInRange(history, end, "1m").length).toBeLessThan(
      history.length,
    );
    expect(creatorMetricsInRange(history, end, "25m")).toEqual(history);
  });

  it("marks a sampling pause as a gap without inventing readings", () => {
    let history = appendCreatorMetric([], point(now));
    history = appendCreatorMetric(history, point(now + 10_000));
    expect(history).toEqual([
      point(now),
      {
        at: now + 2000,
        bitrateKbps: null,
        fps: null,
        segmentMs: null,
        viewers: null,
        chatRate: null,
      },
      point(now + 10_000),
    ]);
    expect(creatorMetricsInRange(history, now + 26 * 60_000, "25m")).toEqual(
      [],
    );
  });
});
