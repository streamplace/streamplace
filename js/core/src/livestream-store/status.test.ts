import { LivestreamViewHydrated, place } from "streamplace";
import { describe, expect, it } from "vitest";
import {
  deriveLivestreamStatus,
  getSegmentMetrics,
  isSegmentFresh,
} from "./status";
import { makeLivestreamStore } from "./store";
import { handleWebSocketMessages } from "./websocket-consumer";

const now = Date.parse("2026-10-26T12:00:00Z");
const segment: place.stream.segment.Main = {
  $type: "place.stream.segment",
  startTime: new Date(now - 1000).toISOString(),
  size: 250_000,
  duration: 1_000_000_000,
  video: [{ width: 1920, height: 1080, framerate: { num: 30_000, den: 1001 } }],
} as place.stream.segment.Main;
const announcement = {
  $type: "place.stream.livestream#livestreamView",
  uri: "at://did:plc:creator/place.stream.livestream/test",
  cid: "test",
  indexedAt: new Date(now).toISOString(),
  author: { did: "did:plc:creator", handle: "creator.example" },
  record: {
    $type: "place.stream.livestream",
    title: "Live title",
    createdAt: new Date(now).toISOString(),
  },
} as LivestreamViewHydrated;

describe("ingest freshness", () => {
  it.each([
    [0, true],
    [9999, true],
    [10_000, false],
    [10_001, false],
    [-1, false],
  ])("uses the existing ten-second threshold at age %s ms", (age, fresh) => {
    expect(isSegmentFresh(new Date(now - age).toISOString(), now)).toBe(fresh);
  });
  it.each([undefined, "", "not a date"])(
    "rejects absent or invalid timestamp %s",
    (time) => {
      expect(isSegmentFresh(time, now)).toBe(false);
    },
  );
});

describe("creator live status", () => {
  it("distinguishes waiting, ready, announced and ended through real store events", () => {
    const store = makeLivestreamStore();
    expect(deriveLivestreamStatus(store.getState(), now)).toMatchObject({
      phase: "connecting",
      incoming: null,
      canAnnounce: false,
      readingsStale: true,
    });
    store.setState({ websocketConnected: true });
    expect(deriveLivestreamStatus(store.getState(), now).phase).toBe("waiting");
    store.setState((state) => handleWebSocketMessages(state, [segment]));
    expect(deriveLivestreamStatus(store.getState(), now)).toMatchObject({
      phase: "ready",
      incoming: true,
      canAnnounce: true,
      readingsStale: false,
    });
    store.setState((state) => handleWebSocketMessages(state, [announcement]));
    expect(deriveLivestreamStatus(store.getState(), now).phase).toBe("live");
    const ended = {
      ...announcement,
      record: { ...announcement.record, endedAt: new Date(now).toISOString() },
    };
    store.setState((state) => handleWebSocketMessages(state, [ended]));
    expect(deriveLivestreamStatus(store.getState(), now)).toMatchObject({
      phase: "ended",
      incoming: true,
      canAnnounce: true,
    });
    expect(store.getState().livestream).toEqual(ended);
  });

  it("marks a disconnected dashboard stale without concluding that OBS or the announcement stopped", () => {
    const store = makeLivestreamStore();
    store.setState((state) =>
      handleWebSocketMessages(state, [segment, announcement]),
    );
    store.setState({ websocketConnected: false });
    expect(deriveLivestreamStatus(store.getState(), now)).toMatchObject({
      phase: "reconnecting",
      incoming: null,
      canAnnounce: false,
      readingsStale: true,
    });
    expect(store.getState().livestream).toEqual(announcement);
    expect(store.getState().segment).toEqual(segment);
  });

  it("retains the active announcement when the last media reading ages out", () => {
    const store = makeLivestreamStore();
    store.setState((state) =>
      handleWebSocketMessages(state, [segment, announcement]),
    );
    expect(deriveLivestreamStatus(store.getState(), now + 9000)).toMatchObject({
      phase: "live",
      incoming: false,
      canAnnounce: false,
      readingsStale: true,
    });
    expect(store.getState().livestream).toEqual(announcement);
  });
});

describe("segment readings", () => {
  it("derives bitrate and frame rate in the dashboard's existing units", () => {
    expect(getSegmentMetrics(segment)).toEqual({
      bitrateKbps: 2000,
      resolution: "1920x1080",
      fps: 30_000 / 1001,
    });
  });
  it.each([0, -1, NaN, Infinity])(
    "does not turn invalid duration %s into a reading",
    (duration) => {
      expect(
        getSegmentMetrics({ ...segment, duration }).bitrateKbps,
      ).toBeNull();
    },
  );
  it.each([undefined, 0, -1, NaN, Infinity])(
    "does not turn invalid size %s into a reading",
    (size) => {
      expect(
        getSegmentMetrics({ ...segment, size } as any).bitrateKbps,
      ).toBeNull();
    },
  );
  it("keeps absent and malformed media values unavailable", () => {
    expect(getSegmentMetrics(null)).toEqual({
      bitrateKbps: null,
      resolution: null,
      fps: null,
    });
    const metrics = getSegmentMetrics({
      ...segment,
      video: [{ width: -1, height: 1080, framerate: { num: 30, den: 0 } }],
    } as any);
    expect(metrics.resolution).toBeNull();
    expect(metrics.fps).toBeNull();
  });
  it.each([0, -1, NaN, Infinity])(
    "rejects invalid frame-rate denominator %s",
    (den) => {
      expect(
        getSegmentMetrics({
          ...segment,
          video: [{ width: 1920, height: 1080, framerate: { num: 30, den } }],
        } as any).fps,
      ).toBeNull();
    },
  );
  it("does not expose overflowing rates or fractional dimensions", () => {
    expect(
      getSegmentMetrics({ ...segment, size: Number.MAX_VALUE }).bitrateKbps,
    ).toBeNull();
    expect(
      getSegmentMetrics({
        ...segment,
        video: [{ width: 1920.5, height: 1080 }],
      } as any).resolution,
    ).toBeNull();
  });
});
