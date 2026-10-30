import { act, createElement, ReactNode, useRef } from "react";
import { createRoot } from "react-dom/client";
import { afterEach, beforeEach, expect, it, vi } from "vitest";
import { VodControls } from "../src/components/mobile-player/ui/vod-controls";
import { NativeVideo } from "../src/components/mobile-player/video-async.native";
import { PlayerContext } from "../src/player-store/context";
import { makePlayerStore } from "../src/player-store/player-store";

const native = vi.hoisted(() => ({ player: null as any }));
vi.mock("expo-video", () => ({
  VideoView: () => createElement("div"),
  useVideoPlayer: (_source: unknown, setup: (player: any) => void) => {
    const ref = useRef<any>(null);
    if (!ref.current) {
      const listeners = new Map<string, Set<(event: any) => void>>();
      ref.current = {
        currentTime: 0,
        duration: 60,
        status: "readyToPlay",
        playing: false,
        addListener(name: string, callback: (event: any) => void) {
          if (!listeners.has(name)) listeners.set(name, new Set());
          listeners.get(name)!.add(callback);
          return { remove: () => listeners.get(name)!.delete(callback) };
        },
        play() {
          this.playing = true;
          queueMicrotask(() => this.emit("playingChange", { isPlaying: true }));
        },
        pause() {
          this.playing = false;
          queueMicrotask(() =>
            this.emit("playingChange", { isPlaying: false }),
          );
        },
        emit(name: string, event: any) {
          listeners.get(name)?.forEach((callback) => callback(event));
        },
      };
      native.player = ref.current;
      setup(ref.current);
    }
    return ref.current;
  },
}));
vi.mock("react-native", () => ({ Linking: {}, Platform: { OS: "ios" } }));
vi.mock("react-native-webrtc", () => ({}));
vi.mock("lucide-react-native", () => ({
  ArrowRight: () => null,
  Pause: () => null,
  Play: () => null,
  RotateCcw: () => null,
}));
vi.mock("../src/components/ui", () => ({
  View: ({ children }: any) => createElement("div", null, children),
  Text: ({ children }: any) => createElement("span", null, children),
  IconButton: ({ children, accessibilityLabel, onPress }: any) =>
    createElement(
      "button",
      { "aria-label": accessibilityLabel, onClick: onPress },
      children,
    ),
}));
vi.mock("../src/player-store", async () => ({
  ...(await import("../src/player-store/player-state")),
  ...(await import("../src/player-store/player-store")),
}));
vi.mock("../src/components/mobile-player/use-webrtc", () => ({}));
vi.mock("../src/components/mobile-player/webrtc-primitives.native", () => ({}));
vi.mock("../src/components/mobile-player/shared", () => ({
  srcToUrl: () => ({ url: "https://media.example/recording.m3u8" }),
}));
vi.mock("../src/livestream-store", () => ({}));
vi.mock("../src/streamplace-store", () => ({}));
vi.mock("../src/storage", () => ({ default: { getItem: async () => null } }));
vi.mock("../src", async () => ({
  ...(await import("../src/player-store/player-state")),
  ...(await import("../src/player-store/player-store")),
  useMuted: () => false,
  useEffectiveVolume: () => 1,
  useStreamplaceStore: () => "https://node.example",
}));

let root: ReturnType<typeof createRoot>;
let container: HTMLDivElement;
let store: ReturnType<typeof makePlayerStore>;
beforeEach(() => {
  vi.stubGlobal("IS_REACT_ACT_ENVIRONMENT", true);
  vi.spyOn(console, "log").mockImplementation(() => {});
  container = document.createElement("div");
  root = createRoot(container);
  store = makePlayerStore("recording");
  store.setState({ mode: "vod", playerEvent: () => {} });
});
afterEach(async () => {
  await act(async () => root.unmount());
  vi.restoreAllMocks();
  vi.useRealTimers();
  vi.unstubAllGlobals();
});
async function render(children?: ReactNode) {
  await act(async () =>
    root.render(
      <PlayerContext.Provider
        value={{
          players: { recording: store },
          createPlayer: () => "recording",
          removePlayer() {},
        }}
      >
        <NativeVideo />
        {children}
      </PlayerContext.Provider>,
    ),
  );
}

it("keeps an intentional recording pause distinct from buffering and resumes", async () => {
  await render();
  expect(store.getState().status).toBe("playing");
  await act(async () => store.getState().togglePlayPause());
  expect(native.player.playing).toBe(false);
  expect(store.getState().status).toBe("pause");
  await act(async () => store.getState().togglePlayPause());
  expect(native.player.playing).toBe(true);
  expect(store.getState().status).toBe("playing");
});

it("keeps live playback waiting when the native player stops", async () => {
  store.setState({ mode: "live" });
  await render();
  await act(async () => native.player.pause());
  expect(store.getState().status).toBe("waiting");
});

it("does not treat a reported playback position as a seek request", async () => {
  await render();
  native.player.currentTime = 26;
  await act(async () => store.getState().setPlayTime(24));
  expect(native.player.currentTime).toBe(26);
});

it("keeps a buffering recording distinct from an intentional pause", async () => {
  await render();
  native.player.status = "loading";
  await act(async () =>
    native.player.emit("playingChange", { isPlaying: false }),
  );
  expect(store.getState().status).toBe("waiting");
});

it.each([
  [60, "readyToPlay"],
  [59.98, "loading"],
] as const)(
  "replays from zero after playback finishes at %s seconds (%s)",
  async (endTime, nativeStatus) => {
    vi.useFakeTimers({ toFake: ["setInterval", "clearInterval"] });
    await render(<VodControls />);
    await act(async () => {
      native.player.currentTime = endTime;
      native.player.status = nativeStatus;
      native.player.pause();
      native.player.emit("playToEnd", {});
      vi.advanceTimersByTime(500);
    });
    expect(store.getState().status).toBe("pause");
    const replay = container.querySelector<HTMLButtonElement>(
      '[aria-label="Replay"]',
    );
    expect(replay).not.toBeNull();
    await act(async () => replay!.click());
    expect(native.player.currentTime).toBe(0);
    expect(native.player.playing).toBe(true);
    expect(store.getState().playTime).toBe(0);
    expect(store.getState().status).toBe("playing");
    expect(container.querySelector('[aria-label="Pause"]')).not.toBeNull();
  },
);

it.each([20, 59.9])(
  "allows seeking to %s in a finished recording before resuming",
  async (time) => {
    vi.useFakeTimers({ toFake: ["setInterval", "clearInterval"] });
    await render(<VodControls />);
    await act(async () => {
      native.player.currentTime = 59.98;
      native.player.pause();
      native.player.emit("playToEnd", {});
      vi.advanceTimersByTime(500);
    });
    expect(container.querySelector('[aria-label="Replay"]')).not.toBeNull();
    await act(async () => store.getState().seekTo(time));
    await act(async () => vi.advanceTimersByTime(500));
    expect(native.player.currentTime).toBe(time);
    expect(store.getState().playTime).toBe(time);
    expect(container.querySelector('[aria-label="Replay"]')).toBeNull();
    await act(async () =>
      container
        .querySelector<HTMLButtonElement>('[aria-label="Play"]')!
        .click(),
    );
    expect(native.player.playing).toBe(true);
    expect(native.player.currentTime).toBe(time);
  },
);

it.each([
  [59.9, 60],
  [0, 0],
])(
  "resumes at %s seconds with duration %s without offering replay",
  async (time, duration) => {
    await render(<VodControls />);
    await act(async () => {
      native.player.currentTime = time;
      native.player.duration = duration;
      native.player.pause();
      store.setState({ duration, playTime: time });
    });
    expect(container.querySelector('[aria-label="Replay"]')).toBeNull();
    const play = container.querySelector<HTMLButtonElement>(
      '[aria-label="Play"]',
    );
    expect(play).not.toBeNull();
    await act(async () => play!.click());
    expect(native.player.currentTime).toBe(time);
    expect(native.player.playing).toBe(true);
  },
);

it("resumes a live stream at its current position", async () => {
  store.setState({ mode: "live" });
  await render();
  await act(async () => {
    native.player.currentTime = 60;
    native.player.pause();
  });
  await act(async () => store.getState().togglePlayPause());
  expect(native.player.currentTime).toBe(60);
  expect(native.player.playing).toBe(true);
});
