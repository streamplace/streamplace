import { describe, expect, it } from "vitest";
import { deriveLiveness, latestActivityAgeSeconds } from "./liveness-state";

const NOW = Date.parse("2026-08-08T18:00:00.000Z");

describe("latestActivityAgeSeconds", () => {
  it("uses the end of the freshest known segment", () => {
    expect(
      latestActivityAgeSeconds(
        {
          segmentStartTime: "2026-08-08T17:59:52.000Z",
          segmentDurationNanoseconds: 5_000_000_000,
          lastSeenAt: "2026-08-08T17:59:40.000Z",
        },
        NOW,
      ),
    ).toBe(3);
  });

  it("uses the livestream heartbeat when it is fresher than the segment", () => {
    expect(
      latestActivityAgeSeconds(
        {
          segmentStartTime: "2026-08-08T17:30:00.000Z",
          lastSeenAt: "2026-08-08T17:59:55.000Z",
        },
        NOW,
      ),
    ).toBe(5);
  });

  it("treats an invalid or future timestamp as fresh", () => {
    expect(latestActivityAgeSeconds({ lastSeenAt: "not-a-date" }, NOW)).toBe(0);
    expect(
      latestActivityAgeSeconds({ lastSeenAt: "2026-08-08T18:00:05.000Z" }, NOW),
    ).toBe(0);
  });
});

describe("deriveLiveness", () => {
  it("stays loading until the websocket sends initial state", () => {
    expect(
      deriveLiveness({
        hasInitialResponse: false,
        hasReceivedSegment: false,
        hasLivestream: false,
        secondsSinceActivity: 0,
      }),
    ).toBe("loading");
  });

  it("reports never-live after initial state has no stream history", () => {
    expect(
      deriveLiveness({
        hasInitialResponse: true,
        hasReceivedSegment: false,
        hasLivestream: false,
        secondsSinceActivity: 0,
      }),
    ).toBe("never-live");
  });

  it("uses activity age for live, stale, and offline boundaries", () => {
    const base = {
      hasInitialResponse: true,
      hasReceivedSegment: true,
      hasLivestream: true,
    };

    expect(deriveLiveness({ ...base, secondsSinceActivity: 9 })).toBe("live");
    expect(deriveLiveness({ ...base, secondsSinceActivity: 10 })).toBe("stale");
    expect(deriveLiveness({ ...base, secondsSinceActivity: 300 })).toBe(
      "offline",
    );
  });

  it("treats an explicit stream ending as immediately offline", () => {
    expect(
      deriveLiveness({
        endedAt: "2026-08-08T17:59:59.000Z",
        hasInitialResponse: true,
        hasReceivedSegment: true,
        hasLivestream: true,
        secondsSinceActivity: 0,
      }),
    ).toBe("offline");
  });
});
