import { act, type RefObject } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { VideoSectionInner } from "./video-section";

const fixture = vi.hoisted(() => ({
  playerRoot: null as HTMLDivElement | null,
}));
vi.mock("../../contexts/fullscreen-context", () => ({
  useFullscreen: () => ({ theatre: false, fullscreen: false }),
}));
vi.mock("../player/hls-player", () => ({ HLSPlayer: () => null }));
vi.mock("../player/webrtc-player", () => ({ WebRTCPlayer: () => null }));
vi.mock("../player/content-warning-overlay", () => ({
  ContentWarningOverlay: () => null,
}));
vi.mock("./user-offline", () => ({ UserOffline: () => null }));
vi.mock("../player/player-controls", () => ({
  PlayerControls: ({
    containerRef,
  }: {
    containerRef: RefObject<HTMLDivElement | null>;
  }) => (
    <button
      onClick={(event) => {
        event.stopPropagation();
        fixture.playerRoot = containerRef.current;
      }}
    >
      player-container
    </button>
  ),
}));
vi.mock("react-i18next", () => ({
  useTranslation: () => ({ t: (key: string) => key }),
}));

describe("video notification overlays", () => {
  let container: HTMLDivElement;
  let root: Root;

  beforeEach(() => {
    vi.stubGlobal("IS_REACT_ACT_ENVIRONMENT", true);
    fixture.playerRoot = null;
    container = document.createElement("div");
    document.body.appendChild(container);
    root = createRoot(container);
  });

  afterEach(async () => {
    await act(async () => root.unmount());
    container.remove();
    vi.unstubAllGlobals();
  });

  async function render(liveness: "live" | "offline" | "never-live") {
    await act(async () =>
      root.render(
        <VideoSectionInner
          user="streamer.example"
          liveness={liveness}
          segment={null}
          problems={[]}
          playlistUrl="https://node.example/playlist.m3u8"
          thumbnailUrl="https://node.example/poster.jpg"
          overlay={<aside>important notice</aside>}
        />,
      ),
    );
    await act(async () =>
      container.querySelector<HTMLButtonElement>("button")?.click(),
    );
  }

  it("keeps the notification inside the player's fullscreen container", async () => {
    await render("live");
    expect(fixture.playerRoot).not.toBeNull();
    expect(fixture.playerRoot?.querySelector("aside")?.textContent).toBe(
      "important notice",
    );
  });

  it.each(["offline", "never-live"] as const)(
    "shows the notification outside the hidden player when %s",
    async (liveness) => {
      await render(liveness);
      const outsidePlayer = Array.from(
        container.querySelectorAll("aside"),
      ).filter((notice) => !fixture.playerRoot?.contains(notice));
      expect(outsidePlayer).toHaveLength(1);
      expect(outsidePlayer[0].textContent).toBe("important notice");
    },
  );
});
