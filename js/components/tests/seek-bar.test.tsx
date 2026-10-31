import { act, createElement } from "react";
import { createRoot } from "react-dom/client";
import { afterEach, beforeEach, expect, it, vi } from "vitest";
import { SeekBar } from "../src/components/mobile-player/ui/seek-bar";
import { PlayerContext } from "../src/player-store/context";
import { makePlayerStore } from "../src/player-store/player-store";

const native = vi.hoisted(() => ({
  views: [] as any[],
  gestures: {} as Record<string, Record<string, (event?: any) => void>>,
}));

// Native layout and accessibility events enter through these adapters. Seeking
// uses the real player store, including its update to the video element.
vi.mock("react-native", () => ({
  View: (props: any) => {
    native.views.push(props);
    return createElement("div", null, props.children);
  },
}));
vi.mock("react-native-gesture-handler", () => {
  // Gesture builders chain configuration and native event handlers.
  const builder = (name: string) => {
    const callbacks: Record<string, (event?: any) => void> = {};
    native.gestures[name] = callbacks;
    const chain = new Proxy(
      {},
      {
        get:
          (_target, key: string) =>
          (...args: any[]) => {
            if (key.startsWith("on")) callbacks[key] = args[0];
            return chain;
          },
      },
    );
    return chain;
  };
  return {
    Gesture: {
      Pan: () => builder("pan"),
      Tap: () => builder("tap"),
      Race: () => ({}),
    },
    GestureDetector: ({ children }: any) => children,
  };
});
vi.mock(
  "../src/player-store",
  () => import("../src/player-store/player-store"),
);
vi.mock("../src/livestream-store", () => ({}));
vi.mock("../src/streamplace-store", () => ({}));
vi.mock("../src/storage", () => ({ default: { getItem: async () => null } }));
vi.mock("../src/components/ui", async () => ({
  useTheme: () => ({ theme: { colors: { primary: "test-accent" } } }),
}));

let container: HTMLDivElement;
let root: ReturnType<typeof createRoot>;
let store: ReturnType<typeof makePlayerStore>;
let video: HTMLVideoElement;
beforeEach(async () => {
  vi.stubGlobal("IS_REACT_ACT_ENVIRONMENT", true);
  native.views = [];
  native.gestures = {};
  container = document.createElement("div");
  root = createRoot(container);
  video = document.createElement("video");
  store = makePlayerStore("recording");
  store.setState({
    mode: "vod",
    duration: 60,
    playTime: 25,
    videoRef: { current: video },
  });
});
afterEach(async () => {
  await act(async () => root.unmount());
  vi.unstubAllGlobals();
});

async function render() {
  await act(async () =>
    root.render(
      <PlayerContext.Provider
        value={{
          players: { recording: store },
          createPlayer: () => "recording",
          removePlayer() {},
        }}
      >
        <SeekBar />
      </PlayerContext.Provider>,
    ),
  );
}

it("exposes an adjustable playback position with a real touch target", async () => {
  await render();
  const slider = native.views.find(
    (view) => view.accessibilityRole === "adjustable",
  );
  expect(slider).toBeDefined();
  expect(slider.accessibilityLabel).toBe("Playback position");
  expect(slider.accessibilityValue).toEqual({ min: 0, max: 60, now: 25 });
  expect(slider.style.height).toBeGreaterThanOrEqual(44);
});

it.each([
  ["increment", 25, 35],
  ["increment", 55, 60],
  ["decrement", 25, 15],
  ["decrement", 5, 0],
  ["activate", 25, 25],
])(
  "handles %s at %s seconds without leaving the recording",
  async (actionName, start, expected) => {
    store.setState({ playTime: start as number });
    video.currentTime = start as number;
    await render();
    const slider = native.views.find(
      (view) => view.accessibilityRole === "adjustable",
    );
    expect(slider).toBeDefined();
    await act(async () =>
      slider.onAccessibilityAction({ nativeEvent: { actionName } }),
    );
    expect(store.getState().playTime).toBe(expected);
    expect(video.currentTime).toBe(expected);
  },
);

it.each([-20, 50, 120])(
  "clamps a tap at %s within the measured track",
  async (x) => {
    await render();
    await act(async () =>
      native.views
        .find((view) => view.onLayout)
        .onLayout({ nativeEvent: { layout: { width: 100 } } }),
    );
    await act(async () => native.gestures.tap.onEnd({ x }));
    expect(store.getState().playTime).toBe(Math.max(0, Math.min(60, x * 0.6)));
    expect(video.currentTime).toBe(store.getState().playTime);
  },
);

it("does not seek when a drag is cancelled", async () => {
  await render();
  await act(async () =>
    native.views
      .find((view) => view.onLayout)
      .onLayout({ nativeEvent: { layout: { width: 100 } } }),
  );
  await act(async () => native.gestures.pan.onBegin({ x: 10 }));
  await act(async () => native.gestures.pan.onUpdate({ x: 80 }));
  await act(async () => native.gestures.pan.onFinalize());
  expect(store.getState().playTime).toBe(25);
});

it.each([0, -1])(
  "omits seeking until a valid duration is available (%s)",
  async (duration) => {
    store.setState({ duration });
    await render();
    expect(container.children).toHaveLength(0);
  },
);
