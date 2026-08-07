import {
  clampWindow,
  dragZone,
  MIN_CLIP_MS,
  moveWindow,
  msToPx,
  pxToMs,
  resizeWindow,
} from "./trim-math";

describe("msToPx / pxToMs", () => {
  it("maps ms to px proportionally", () => {
    expect(msToPx(30000, 120000, 600)).toBe(150);
    expect(pxToMs(150, 120000, 600)).toBe(30000);
  });

  it("round-trips", () => {
    expect(pxToMs(msToPx(12345, 120000, 600), 120000, 600)).toBeCloseTo(12345);
  });

  it("handles zero duration/width", () => {
    expect(msToPx(1000, 0, 600)).toBe(0);
    expect(pxToMs(100, 120000, 0)).toBe(0);
  });
});

describe("clampWindow", () => {
  it("clamps to the track bounds", () => {
    expect(clampWindow(-5000, 125000, 120000)).toEqual({
      start: 0,
      end: 120000,
    });
  });

  it("normalizes inverted edges", () => {
    expect(clampWindow(90000, 10000, 120000)).toEqual({
      start: 10000,
      end: 90000,
    });
  });

  it("enforces the minimum window by pushing the far boundary", () => {
    // [18000, 20000] is 2s; min 5s; room after the end → extend the end.
    expect(clampWindow(18000, 20000, 120000, 5000)).toEqual({
      start: 18000,
      end: 23000,
    });
  });

  it("pushes the near boundary when the far edge is at the track end", () => {
    // [118000, 120000] is 2s; min 5s; no room at the end → extend the start.
    expect(clampWindow(118000, 120000, 120000, 5000)).toEqual({
      start: 115000,
      end: 120000,
    });
  });

  it("uses the whole track when the track is shorter than the minimum", () => {
    expect(clampWindow(1000, 2000, 3000, 5000)).toEqual({
      start: 0,
      end: 3000,
    });
  });

  it("treats a zero-duration track as empty", () => {
    expect(clampWindow(0, 0, 0)).toEqual({ start: 0, end: 0 });
  });

  it("keeps the full duration as the default window", () => {
    expect(clampWindow(0, 120000, 120000)).toEqual({ start: 0, end: 120000 });
  });
});

describe("moveWindow", () => {
  it("moves the window by the delta", () => {
    expect(moveWindow(30000, 60000, 10000, 120000)).toEqual({
      start: 40000,
      end: 70000,
    });
  });

  it("clamps at the left edge, preserving size", () => {
    expect(moveWindow(30000, 60000, -40000, 120000)).toEqual({
      start: 0,
      end: 30000,
    });
  });

  it("clamps at the right edge, preserving size", () => {
    expect(moveWindow(30000, 60000, 70000, 120000)).toEqual({
      start: 90000,
      end: 120000,
    });
  });
});

describe("resizeWindow", () => {
  it("resizes from the left edge", () => {
    expect(resizeWindow("left", 30000, 60000, 40000, 120000)).toEqual({
      start: 40000,
      end: 60000,
    });
  });

  it("resizes from the right edge", () => {
    expect(resizeWindow("right", 30000, 60000, 50000, 120000)).toEqual({
      start: 30000,
      end: 50000,
    });
  });

  it("never shrinks below the minimum window", () => {
    // Left drag to 58s would make a 2s window → the handle stops at 55s.
    expect(resizeWindow("left", 30000, 60000, 58000, 120000, 5000)).toEqual({
      start: 55000,
      end: 60000,
    });
    // Right drag to 32s would make a 2s window → the handle stops at 35s.
    expect(resizeWindow("right", 30000, 60000, 32000, 120000, 5000)).toEqual({
      start: 30000,
      end: 35000,
    });
  });

  it("clamps edges to the track", () => {
    expect(resizeWindow("left", 30000, 60000, -5000, 120000)).toEqual({
      start: 0,
      end: 60000,
    });
    expect(resizeWindow("right", 30000, 60000, 200000, 120000)).toEqual({
      start: 30000,
      end: 120000,
    });
  });
});

describe("dragZone", () => {
  const left = 100;
  const right = 300;
  const hw = 12;

  it("returns left for the left handle zone", () => {
    expect(dragZone(95, left, right, hw)).toBe("left");
    expect(dragZone(112, left, right, hw)).toBe("left");
  });

  it("returns right for the right handle zone", () => {
    expect(dragZone(295, left, right, hw)).toBe("right");
    expect(dragZone(312, left, right, hw)).toBe("right");
  });

  it("returns body inside the window", () => {
    expect(dragZone(150, left, right, hw)).toBe("body");
  });

  it("returns none outside the window", () => {
    expect(dragZone(50, left, right, hw)).toBe("none");
    expect(dragZone(400, left, right, hw)).toBe("none");
  });
});

describe("MIN_CLIP_MS", () => {
  it("is 5 seconds", () => {
    expect(MIN_CLIP_MS).toBe(5000);
  });
});
