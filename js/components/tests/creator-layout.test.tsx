import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import {
  DEFAULT_LAYOUT,
  getDropSide,
  getWidgetPlacements,
  insertWidget,
  moveWidget,
  removeWidget,
  updateRatio,
  validateLayout,
} from "../../app/components/live-dashboard/creator-layout";
import {
  loadLayout,
  loadPresets,
  saveLayout,
  savePresets,
} from "../../app/components/live-dashboard/creator-layout-storage";

const monitor = {
  type: "leaf",
  widget: "stream-monitor",
  config: { muted: true },
} as const;
const chat = {
  type: "leaf",
  widget: "chat",
  config: { filter: "following" },
} as const;
const health = { type: "leaf", widget: "stream-health" } as const;
const split = {
  type: "split",
  direction: "horizontal",
  ratio: [2, 1],
  children: [
    monitor,
    {
      type: "split",
      direction: "vertical",
      ratio: [1, 3],
      children: [chat, health],
    },
  ],
} as const;

describe("creator layout validation", () => {
  it("normalizes ratios without changing widget configuration or unknown keys", () => {
    const value = {
      ...split,
      children: [
        monitor,
        { type: "leaf", widget: "future-widget", config: { version: 2 } },
      ],
    };
    expect(validateLayout(value)).toEqual({
      ...value,
      ratio: [2 / 3, 1 / 3],
    });
    expect(value.ratio).toEqual([2, 1]);
  });

  it("gives an all-zero split equal sizes", () => {
    expect(validateLayout({ ...split, ratio: [0, 0] })).toMatchObject({
      ratio: [0.5, 0.5],
    });
  });

  it("normalizes very large finite ratios without overflowing", () => {
    expect(
      validateLayout({ ...split, ratio: [Number.MAX_VALUE, Number.MAX_VALUE] }),
    ).toMatchObject({ ratio: [0.5, 0.5] });
  });

  it.each([
    null,
    { type: "leaf", widget: 1 },
    { type: "leaf", widget: "chat", config: [] },
    { ...split, children: [monitor, { type: "leaf", widget: 1 }] },
    { ...split, ratio: [1] },
    { ...split, ratio: [1, -1] },
    { ...split, ratio: [1, Number.NaN] },
    { ...split, ratio: [1, Number.POSITIVE_INFINITY] },
    { ...split, direction: "diagonal" },
  ])(
    "rejects invalid layouts rather than misaligning children and ratios: %o",
    (value) => {
      expect(validateLayout(value)).toBeNull();
    },
  );
});

describe("creator layout edits", () => {
  it("chooses the split side from the pointer position", () => {
    expect(getDropSide(0.1, 0.5)).toEqual({
      direction: "horizontal",
      first: true,
    });
    expect(getDropSide(0.9, 0.5)).toEqual({
      direction: "horizontal",
      first: false,
    });
    expect(getDropSide(0.5, 0.1)).toEqual({
      direction: "vertical",
      first: true,
    });
    expect(getDropSide(0.5, 0.9)).toEqual({
      direction: "vertical",
      first: false,
    });
    expect(getDropSide(0, 0)).toEqual({ direction: "horizontal", first: true });
    expect(getDropSide(-0.1, 0.5)).toBeNull();
    expect(getDropSide(0.5, Number.NaN)).toBeNull();
    expect(getDropSide(Number.POSITIVE_INFINITY, 0.5)).toBeNull();
  });
  it("places nested leaves in their saved order and proportions", () => {
    const layout = validateLayout(split)!;
    expect(getWidgetPlacements(layout)).toEqual([
      { widget: monitor, path: [0], x: 0, y: 0, width: 2 / 3, height: 1 },
      {
        widget: chat,
        path: [1, 0],
        x: 2 / 3,
        y: 0,
        width: 1 / 3,
        height: 0.25,
      },
      {
        widget: health,
        path: [1, 1],
        x: 2 / 3,
        y: 0.25,
        width: 1 / 3,
        height: 0.75,
      },
    ]);
  });
  it("inserts beside a nested leaf without discarding its configuration", () => {
    const original = validateLayout(split)!;
    const result = insertWidget(
      original,
      [1, 0],
      { type: "leaf", widget: "moderators" },
      "horizontal",
      false,
    );
    expect(result).toEqual({
      ...original,
      children: [
        monitor,
        {
          type: "split",
          direction: "vertical",
          ratio: [0.25, 0.75],
          children: [
            {
              type: "split",
              direction: "horizontal",
              ratio: [0.5, 0.5],
              children: [chat, { type: "leaf", widget: "moderators" }],
            },
            health,
          ],
        },
      ],
    });
    expect(original).toEqual(validateLayout(split));
  });

  it("moves a configured widget and collapses its old split", () => {
    const original = validateLayout(split)!;
    const result = moveWidget(original, [1, 0], [0], "vertical", true);
    expect(result).toEqual({
      type: "split",
      direction: "horizontal",
      ratio: [2 / 3, 1 / 3],
      children: [
        {
          type: "split",
          direction: "vertical",
          ratio: [0.5, 0.5],
          children: [chat, monitor],
        },
        health,
      ],
    });
  });

  it("removes a leaf and renormalizes surviving siblings", () => {
    const original = validateLayout({
      type: "split",
      direction: "vertical",
      ratio: [2, 3, 5],
      children: [monitor, chat, health],
    })!;
    const result = removeWidget(original, [1]);
    expect(result).toMatchObject({
      type: "split",
      direction: "vertical",
      children: [monitor, health],
    });
    expect(result.type).toBe("split");
    if (result.type === "split") {
      expect(result.ratio[0]).toBeCloseTo(2 / 7);
      expect(result.ratio[1]).toBeCloseTo(5 / 7);
    }
    expect(removeWidget(validateLayout(split)!, [1, 0])).toEqual({
      type: "split",
      direction: "horizontal",
      ratio: [2 / 3, 1 / 3],
      children: [monitor, health],
    });
  });

  it("leaves an empty slot when removing the final widget and never removes a whole branch", () => {
    const original = validateLayout(split)!;
    expect(removeWidget(monitor, [])).toEqual({
      type: "leaf",
      widget: "blank",
    });
    expect(removeWidget(original, [1])).toBe(original);
  });

  it.each([
    { path: [-1] },
    { path: [1.5] },
    { path: [2] },
    { path: [0, 0] },
    { path: [Number.NaN] },
  ])("ignores invalid paths $path", ({ path }) => {
    const original = validateLayout(split)!;
    expect(removeWidget(original, path)).toBe(original);
    expect(insertWidget(original, path, health, "vertical", false)).toBe(
      original,
    );
    expect(moveWidget(original, path, [0], "vertical", false)).toBe(original);
  });

  it("rejects invalid resize ratios and normalizes valid ones", () => {
    const original = validateLayout(split)!;
    expect(updateRatio(original, [], [1, -1])).toBe(original);
    expect(updateRatio(original, [], [1])).toBe(original);
    expect(updateRatio(original, [], [1, 3])).toEqual({
      ...original,
      ratio: [0.25, 0.75],
    });
  });
});

describe("creator layout storage", () => {
  beforeEach(() => localStorage.clear());
  afterEach(() => vi.unstubAllGlobals());

  it("reads existing web layouts and round-trips them under the same key", () => {
    localStorage.setItem("dashboard:layout", JSON.stringify(split));
    const normalized = validateLayout(split)!;
    expect(loadLayout()).toEqual(normalized);
    expect(saveLayout(normalized)).toBe(true);
    expect(JSON.parse(localStorage.getItem("dashboard:layout")!)).toEqual(
      normalized,
    );
  });

  it("uses a safe default for malformed stored layouts without overwriting them", () => {
    const bad = JSON.stringify({
      ...split,
      children: [monitor, { type: "bad" }],
    });
    localStorage.setItem("dashboard:layout", bad);
    expect(loadLayout()).toEqual(DEFAULT_LAYOUT);
    expect(localStorage.getItem("dashboard:layout")).toBe(bad);
    localStorage.setItem("dashboard:layout", "{broken");
    expect(loadLayout()).toEqual(DEFAULT_LAYOUT);
  });

  it("loads normalized presets and ignores malformed entries", () => {
    localStorage.setItem(
      "dashboard:presets",
      JSON.stringify([
        { name: "old", layout: split },
        { name: "bad", layout: { ...split, ratio: [1, -1] } },
        { name: 4, layout: split },
      ]),
    );
    const presets = [{ name: "old", layout: validateLayout(split)! }];
    expect(loadPresets()).toEqual(presets);
    expect(savePresets(presets)).toBe(true);
    expect(loadPresets()).toEqual(presets);
  });

  it("ignores malformed preset storage", () => {
    localStorage.setItem("dashboard:presets", "{broken");
    expect(loadPresets()).toEqual([]);
    localStorage.setItem(
      "dashboard:presets",
      JSON.stringify({ name: "not a list" }),
    );
    expect(loadPresets()).toEqual([]);
  });

  it("survives storage access failures and reports unsuccessful saves", () => {
    vi.stubGlobal("localStorage", {
      getItem() {
        throw new Error("storage unavailable");
      },
      setItem() {
        throw new Error("storage unavailable");
      },
    });
    expect(loadLayout()).toEqual(DEFAULT_LAYOUT);
    expect(loadPresets()).toEqual([]);
    expect(saveLayout(DEFAULT_LAYOUT)).toBe(false);
    expect(savePresets([])).toBe(false);
  });
});
