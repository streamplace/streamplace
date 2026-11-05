import { act, createElement, useRef } from "react";
import { createRoot } from "react-dom/client";
import { afterEach, beforeEach, expect, it, vi } from "vitest";
import { DesktopUi } from "../../app/components/mobile/desktop-ui";
import { PlayerContext } from "../src/player-store/context";
import { PlayerStatus } from "../src/player-store/player-state";
import { makePlayerStore } from "../src/player-store/player-store";

const input = vi.hoisted(() => ({
  ingest: null as string | null,
  tap: {} as Record<number, (event: unknown, success: boolean) => void>,
  hover: () => {},
}));

function View({
  children,
  style,
  pointerEvents,
  testID,
  onFocus,
  onBlur,
  onPointerDown,
  "aria-hidden": ariaHidden,
}: any) {
  const flattenedStyle = Object.assign({}, ...[style].flat(Infinity));
  return createElement(
    "div",
    {
      "data-testid": testID,
      "data-pointer-events": pointerEvents ?? flattenedStyle.pointerEvents,
      "aria-hidden": ariaHidden,
      onFocus,
      onBlur,
      onPointerDown: (event: PointerEvent) =>
        onPointerDown?.({ nativeEvent: { pointerType: event.pointerType } }),
    },
    children,
  );
}

// Only native presentation and input bindings are adapted. Player state,
// playback actions, focus handling, and visibility timing remain real.
vi.mock("react-native", () => ({
  Platform: { OS: "web", select: () => null },
}));
vi.mock("react-native-reanimated", () => ({
  default: { View },
  runOnJS: (callback: unknown) => callback,
  useSharedValue: (value: unknown) => useRef({ value }).current,
  useAnimatedStyle: (style: () => unknown) => style(),
  withTiming: (value: unknown) => value,
}));
vi.mock("react-native-gesture-handler", () => ({
  Gesture: {
    Hover: () => ({
      onChange(callback: () => void) {
        input.hover = callback;
        return this;
      },
    }),
    Tap: () => ({
      taps: 1,
      numberOfTaps(taps: number) {
        this.taps = taps;
        return this;
      },
      onEnd(callback: (event: unknown, success: boolean) => void) {
        input.tap[this.taps] = callback;
        return this;
      },
    }),
    Exclusive: () => ({}),
    Race: () => ({}),
  },
  GestureDetector: ({ children }: any) => children,
}));
vi.mock("react-native-safe-area-context", () => ({
  useSafeAreaInsets: () => ({ top: 0, bottom: 0, left: 0, right: 0 }),
}));
vi.mock("components/ui/gradient", () => ({ default: View }));
vi.mock("../../app/components/mobile/desktop-ui/play-pause-indicator", () => ({
  PlayPauseIndicator: () => null,
}));
vi.mock("../../app/components/mobile/desktop-ui/index", () => ({
  TopControlBar: () => <button>Header action</button>,
  MuteOverlay: () => null,
  BottomControlBar: ({ onMenuOpenChange, onScrubbingChange }: any) => (
    <>
      <button>Footer action</button>
      <button onClick={() => onMenuOpenChange?.(true)}>Open settings</button>
      <button onClick={() => onMenuOpenChange?.(false)}>Close settings</button>
      <button onClick={() => onScrubbingChange?.(true)}>Begin seeking</button>
      <button onClick={() => onScrubbingChange?.(false)}>End seeking</button>
    </>
  ),
}));
vi.mock("../src/livestream-store", () => ({}));
vi.mock("../src/streamplace-store", () => ({}));
vi.mock("../src/storage", () => ({ default: { getItem: async () => null } }));
vi.mock("@streamplace/components", async () => ({
  ...(await import("../src/player-store/player-state")),
  ...(await import("../src/player-store/player-store")),
  zero: await import("../src/lib/theme/atoms"),
  View,
  useOffline: () => false,
  usePlayerDimensions: () => ({ width: 1280, height: 720 }),
  useLivestreamInfo: () => ({ ingest: input.ingest }),
  PortalHost: () => null,
  Toast: () => null,
  PlayerUI: {
    ViewerLoadingOverlay: () => null,
    CountdownOverlay: () => null,
    InputPanel: () => null,
    MetricsPanel: () => null,
  },
}));

let container: HTMLDivElement;
let root: ReturnType<typeof createRoot>;
let store: ReturnType<typeof makePlayerStore>;
let video: HTMLVideoElement;
beforeEach(() => {
  vi.stubGlobal("IS_REACT_ACT_ENVIRONMENT", true);
  vi.useFakeTimers();
  input.ingest = null;
  input.tap = {};
  container = document.createElement("div");
  document.body.appendChild(container);
  root = createRoot(container);
  store = makePlayerStore("desktop-player");
  let paused = false;
  video = document.createElement("video");
  Object.defineProperty(video, "paused", { get: () => paused });
  video.pause = () => {
    paused = true;
    store.getState().setStatus(PlayerStatus.PAUSE);
  };
  video.play = async () => {
    paused = false;
    store.getState().setStatus(PlayerStatus.PLAYING);
  };
  store.setState({
    mode: "vod",
    status: PlayerStatus.PLAYING,
    videoRef: { current: video },
  });
});
afterEach(async () => {
  await act(async () => root.unmount());
  container.remove();
  vi.useRealTimers();
  vi.unstubAllGlobals();
});

async function render() {
  await act(async () =>
    root.render(
      <PlayerContext.Provider
        value={{
          players: { "desktop-player": store },
          createPlayer: () => "desktop-player",
          removePlayer() {},
        }}
      >
        <DesktopUi />
      </PlayerContext.Provider>,
    ),
  );
}
function button(label: string) {
  return [...container.querySelectorAll("button")].find(
    (node) => node.textContent === label,
  )!;
}
function interactive(element: HTMLElement) {
  return !element.closest('[data-pointer-events="none"]');
}
async function wait(ms: number) {
  await act(async () => vi.advanceTimersByTime(ms));
}
async function pointer(type: string, element?: HTMLElement) {
  const event = new Event("pointerdown", { bubbles: true });
  Object.defineProperty(event, "pointerType", { value: type });
  await act(async () =>
    (
      element ??
      container.querySelector('[data-testid="desktop-player-surface"]')!
    ).dispatchEvent(event),
  );
}
async function tap(success = true) {
  await act(async () => input.tap[1]({}, success));
}

it("gives header and footer six seconds, then disables both until revealed", async () => {
  await render();
  await wait(5900);
  expect(interactive(button("Header action"))).toBe(true);
  expect(interactive(button("Footer action"))).toBe(true);
  await wait(200);
  expect(interactive(button("Header action"))).toBe(false);
  expect(interactive(button("Footer action"))).toBe(false);
  expect(
    button("Header action").closest('[aria-hidden="true"]'),
  ).not.toBeNull();
  expect(
    button("Footer action").closest('[aria-hidden="true"]'),
  ).not.toBeNull();
  await act(async () => input.hover());
  expect(interactive(button("Header action"))).toBe(true);
  expect(interactive(button("Footer action"))).toBe(true);
  expect(button("Header action").closest('[aria-hidden="true"]')).toBeNull();
  expect(button("Footer action").closest('[aria-hidden="true"]')).toBeNull();
});

it.each(["paused", "ended"])(
  "keeps %s VOD controls available",
  async (state) => {
    store.setState(
      state === "paused"
        ? { status: PlayerStatus.PAUSE }
        : { status: PlayerStatus.SUSPEND, duration: 60, playTime: 60 },
    );
    await render();
    await wait(7000);
    expect(interactive(button("Header action"))).toBe(true);
    expect(interactive(button("Footer action"))).toBe(true);
  },
);

it.each([
  ["Open settings", "Close settings"],
  ["Begin seeking", "End seeking"],
])(
  "holds controls during %s and starts a fresh deadline afterwards",
  async (open, close) => {
    await render();
    await act(async () => button(open).click());
    await wait(7000);
    expect(interactive(button("Header action"))).toBe(true);
    expect(interactive(button("Footer action"))).toBe(true);
    await act(async () => button(close).click());
    await wait(5900);
    expect(interactive(button("Footer action"))).toBe(true);
    await wait(200);
    expect(interactive(button("Header action"))).toBe(false);
    expect(interactive(button("Footer action"))).toBe(false);
  },
);

it("holds keyboard focus across control groups and releases it on exit", async () => {
  await render();
  await act(async () =>
    document.dispatchEvent(new KeyboardEvent("keydown", { key: "Tab" })),
  );
  await act(async () => button("Footer action").focus());
  await wait(7000);
  expect(interactive(button("Footer action"))).toBe(true);
  await act(async () => button("Header action").focus());
  await wait(7000);
  expect(interactive(button("Header action"))).toBe(true);
  const outside = document.createElement("button");
  container.appendChild(outside);
  await act(async () => outside.focus());
  await wait(6100);
  expect(interactive(button("Header action"))).toBe(false);
  expect(interactive(button("Footer action"))).toBe(false);
});

it("does not let ordinary mouse focus keep playback chrome visible forever", async () => {
  await render();
  await pointer("mouse", button("Footer action"));
  await act(async () => button("Footer action").focus());
  await wait(6100);
  expect(interactive(button("Footer action"))).toBe(false);
  await act(async () =>
    document.dispatchEvent(new KeyboardEvent("keydown", { key: "ArrowLeft" })),
  );
  await wait(7000);
  expect(interactive(button("Footer action"))).toBe(true);
});

it("uses the first hidden mouse click to reveal and a visible VOD click to pause", async () => {
  await render();
  await wait(6100);
  await pointer("mouse");
  await tap();
  expect(video.paused).toBe(false);
  expect(interactive(button("Header action"))).toBe(true);
  await pointer("mouse");
  await tap();
  expect(video.paused).toBe(true);
  expect(store.getState().status).toBe(PlayerStatus.PAUSE);
});

it.each([
  ["live", false],
  ["vod", false],
  ["vod", true],
] as const)(
  "touch taps toggle %s chrome (paused: %s) without changing playback",
  async (mode, paused) => {
    store.setState({ mode });
    if (paused) video.pause();
    await render();
    await pointer("touch");
    await tap(false);
    expect(interactive(button("Header action"))).toBe(true);
    await tap();
    expect(interactive(button("Header action"))).toBe(false);
    expect(interactive(button("Footer action"))).toBe(false);
    expect(video.paused).toBe(paused);
    await pointer("touch");
    await tap();
    expect(interactive(button("Header action"))).toBe(true);
    expect(interactive(button("Footer action"))).toBe(true);
    expect(video.paused).toBe(paused);
  },
);

it("preserves F and double-click fullscreen while ignoring F in a text input", async () => {
  await render();
  await act(async () =>
    document.dispatchEvent(new KeyboardEvent("keydown", { key: "f" })),
  );
  expect(store.getState().fullscreen).toBe(true);
  await act(async () => input.tap[2]({}, false));
  expect(store.getState().fullscreen).toBe(true);
  await act(async () => input.tap[2]({}, true));
  expect(store.getState().fullscreen).toBe(false);
  const field = document.createElement("input");
  container.appendChild(field);
  field.focus();
  await act(async () =>
    document.dispatchEvent(new KeyboardEvent("keydown", { key: "f" })),
  );
  expect(store.getState().fullscreen).toBe(false);
});

it.each([null, "new", "connected"])(
  "keeps %s ingest or audio-only controls available",
  async (ingest) => {
    input.ingest = ingest;
    if (ingest === null) store.setState({ selectedRendition: "audio" });
    await render();
    await wait(7000);
    expect(interactive(button("Header action"))).toBe(true);
    expect(interactive(button("Footer action"))).toBe(true);
  },
);
