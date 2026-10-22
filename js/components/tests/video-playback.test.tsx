import { act } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { PlayerKeyboardShortcuts } from "../src/components/mobile-player/ui/player-keyboard-shortcuts";
import { ProgressiveMP4Player } from "../src/components/mobile-player/video";
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
vi.mock("../src/storage", () => ({
  default: { getItem: async () => null, setItem: async () => {} },
}));

vi.mock("../src/components/ui/index", () => ({
  Text: () => null,
  View: () => null,
}));
vi.mock("../src/components/ui/loader", () => ({ Loader: () => null }));
vi.mock("../src/components/mobile-player/use-webrtc", () => ({
  default: () => {},
  useWebRTCIngest: () => {},
}));
vi.mock("../src/components/mobile-player/webrtc-diagnostics", () => ({
  logWebRTCDiagnostics: () => {},
  useWebRTCDiagnostics: () => ({}),
}));
vi.mock("../src/components/mobile-player/webrtc-primitives", () => ({
  checkWebRTCSupport: () => {},
}));

vi.mock("../src", async () => {
  const [{ usePlayerStore }, { PlayerProtocol, PlayerStatus }] =
    await Promise.all([
      vi.importActual<typeof import("../src/player-store/player-store")>(
        "../src/player-store/player-store",
      ),
      vi.importActual<typeof import("../src/player-store/player-state")>(
        "../src/player-store/player-state",
      ),
    ]);

  return {
    PlayerProtocol,
    PlayerStatus,
    usePlayerStore,
    useEffectiveVolume: () => 1,
    useMuted: () => false,
    useSetMuted: () => () => {},
    useStreamplaceStore: (selector: (state: { url: string }) => unknown) =>
      selector({ url: "" }),
  };
});

describe("video playback around paused seeks", () => {
  const playerId = "paused-seek-test";
  let root: Root;
  let host: HTMLDivElement;
  let store: ReturnType<typeof makePlayerStore>;
  let video: HTMLVideoElement;
  let paused: boolean;

  beforeEach(async () => {
    vi.stubGlobal("IS_REACT_ACT_ENVIRONMENT", true);
    paused = false;
    store = makePlayerStore(playerId);
    store.getState().setMode("vod");
    store.getState().setStatus(PlayerStatus.PLAYING);
    store.getState().setDuration(120);
    store.getState().setSelectedRendition("1080p");
    store.getState().setVodLevels([{ name: "1080p", frameRate: 60 }]);
    store.setState({ playerEvent: () => {} });

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
          <>
            <ProgressiveMP4Player url="/video.mp4" />
            <PlayerKeyboardShortcuts />
          </>
        </PlayerContext.Provider>,
      );
    });

    video = host.querySelector("video")!;
    Object.defineProperties(video, {
      duration: { configurable: true, value: 120 },
      paused: { configurable: true, get: () => paused },
    });
    vi.spyOn(video, "play").mockImplementation(async () => {
      paused = false;
    });
    vi.spyOn(video, "pause").mockImplementation(() => {
      paused = true;
      video.dispatchEvent(new Event("pause"));
    });
  });

  afterEach(async () => {
    await act(async () => root.unmount());
    host.remove();
    vi.restoreAllMocks();
    vi.unstubAllGlobals();
  });

  it("does not autoplay after a paused frame seek becomes playable", async () => {
    await act(async () => {
      video.pause();
    });
    expect(store.getState().status).toBe(PlayerStatus.PAUSE);

    const timeBeforeSeek = video.currentTime;
    await act(async () => {
      window.dispatchEvent(
        new KeyboardEvent("keydown", { key: ".", cancelable: true }),
      );
    });
    expect(video.currentTime).toBeGreaterThan(timeBeforeSeek);

    await act(async () => {
      video.dispatchEvent(new Event("canplay"));
      video.dispatchEvent(new Event("canplaythrough"));
    });

    expect(video.paused).toBe(true);
    expect(video.play).not.toHaveBeenCalled();
    expect(store.getState().status).toBe(PlayerStatus.PAUSE);
  });

  it("attempts autoplay once when the video first becomes playable", async () => {
    await act(async () => {
      video.dispatchEvent(new Event("canplay"));
      video.dispatchEvent(new Event("canplaythrough"));
    });

    expect(video.play).toHaveBeenCalledTimes(1);
  });
});
