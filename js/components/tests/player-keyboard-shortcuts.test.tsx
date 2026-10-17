import { act } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { PlayerKeyboardShortcuts } from "../src/components/mobile-player/ui/player-keyboard-shortcuts";
import { PlayerContext } from "../src/player-store/context";
import { PlayerStatus } from "../src/player-store/player-state";
import { makePlayerStore } from "../src/player-store/player-store";

vi.hoisted(() => {
  const buffer = (globalThis as { Buffer?: typeof Buffer }).Buffer;
  if (!buffer) return;

  const uint8Array = Uint8Array as typeof Uint8Array & {
    fromBase64?: (value: string) => Uint8Array;
  };
  if (!uint8Array.fromBase64) {
    uint8Array.fromBase64 = (value) =>
      new Uint8Array(buffer.from(value, "base64"));
  }
  const uint8ArrayPrototype = Uint8Array.prototype as Uint8Array & {
    toBase64?: () => string;
  };
  if (!uint8ArrayPrototype.toBase64) {
    uint8ArrayPrototype.toBase64 = function () {
      return buffer.from(this).toString("base64");
    };
  }
});

vi.mock("react-native", () => ({ Platform: { OS: "web" } }));

describe("app player keyboard shortcuts", () => {
  const playerId = "keyboard-shortcuts-test";
  let root: Root;
  let host: HTMLDivElement;
  let video: HTMLVideoElement;
  let store: ReturnType<typeof makePlayerStore>;
  let paused: boolean;

  beforeEach(async () => {
    vi.stubGlobal("IS_REACT_ACT_ENVIRONMENT", true);
    paused = true;
    video = document.createElement("video");
    Object.defineProperties(video, {
      duration: { configurable: true, value: 120 },
      paused: { configurable: true, get: () => paused },
    });
    vi.spyOn(video, "play").mockImplementation(async () => {
      paused = false;
    });
    vi.spyOn(video, "pause").mockImplementation(() => {
      paused = true;
    });
    video.currentTime = 10;

    store = makePlayerStore(playerId);
    store.getState().setMode("vod");
    store.getState().setStatus(PlayerStatus.PAUSE);
    store.getState().setDuration(120);
    store.getState().setSelectedRendition("1080p");
    store.getState().setVodLevels([{ name: "1080p", frameRate: 60 }]);
    store.getState().setVideoRef({ current: video });

    host = document.createElement("div");
    document.body.append(host);
    root = createRoot(host);
    await act(async () => {
      root.render(
        <PlayerContext.Provider
          value={{
            players: { [playerId]: store },
            createPlayer: () => playerId,
            removePlayer: () => {},
          }}
        >
          <PlayerKeyboardShortcuts />
        </PlayerContext.Provider>,
      );
    });
  });

  afterEach(async () => {
    await act(async () => root.unmount());
    host.remove();
    vi.restoreAllMocks();
    vi.unstubAllGlobals();
  });

  async function press(
    key: string,
    target: EventTarget = document.body,
    options: KeyboardEventInit = {},
  ) {
    const event = new KeyboardEvent("keydown", {
      key,
      bubbles: true,
      cancelable: true,
      ...options,
    });
    await act(async () => target.dispatchEvent(event));
    return event;
  }

  it("toggles playback with space and ignores repeated keydown events", async () => {
    expect((await press(" ")).defaultPrevented).toBe(true);
    expect(video.paused).toBe(false);
    expect(video.play).toHaveBeenCalledTimes(1);

    await press(" ", document.body, { repeat: true });
    expect(video.paused).toBe(false);
    expect(video.pause).not.toHaveBeenCalled();

    await press(" ");
    expect(video.paused).toBe(true);
    expect(video.pause).toHaveBeenCalledTimes(1);
  });

  it("leaves spacebar alone when typing into an input", async () => {
    const input = document.createElement("input");
    document.body.append(input);

    const event = await press(" ", input);

    expect(event.defaultPrevented).toBe(false);
    expect(video.paused).toBe(true);
    input.remove();
  });

  it("steps one advertised frame in either direction while paused", async () => {
    await press(".");
    expect(video.currentTime).toBeCloseTo(10 + 1 / 60);

    await press(",");
    expect(video.currentTime).toBeCloseTo(10);
  });

  it("steps frames when a player control still has focus", async () => {
    const pauseButton = document.createElement("button");
    document.body.append(pauseButton);
    pauseButton.focus();

    await press(".", pauseButton);

    expect(video.currentTime).toBeCloseTo(10 + 1 / 60);
    pauseButton.remove();
  });

  it("does not step frames while playing", async () => {
    await act(async () => {
      store.getState().setStatus(PlayerStatus.PLAYING);
    });

    const event = await press(".");

    expect(event.defaultPrevented).toBe(false);
    expect(video.currentTime).toBe(10);
  });
});
