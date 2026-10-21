import { act, createElement } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeEach, expect, it, vi } from "vitest";
import { srcToUrl } from "../src/components/mobile-player/shared";
import { PlayerProtocol } from "../src/player-store/player-state";

vi.mock("../src/streamplace-store", () => ({
  useStreamplaceStore: <T,>(select: (state: { url: string }) => T) =>
    select({ url: "https://node.example" }),
}));
vi.mock("../src/player-store", () => ({
  usePlayerStore: <T,>(select: (state: { playbackSession: undefined }) => T) =>
    select({ playbackSession: undefined }),
}));

const DID = "did:web:streamer.example%3A8443";
let container: HTMLDivElement;
let root: Root;

beforeEach(() => {
  vi.stubGlobal("IS_REACT_ACT_ENVIRONMENT", true);
  container = document.createElement("div");
  document.body.append(container);
  root = createRoot(container);
});
afterEach(async () => {
  await act(async () => root.unmount());
  container.remove();
  vi.unstubAllGlobals();
});

function PlaybackURL({
  src,
  protocol,
}: {
  src: string;
  protocol: PlayerProtocol;
}) {
  return createElement("output", null, srcToUrl({ src }, protocol).url);
}

it.each([
  PlayerProtocol.HLS,
  PlayerProtocol.PROGRESSIVE_MP4,
  PlayerProtocol.PROGRESSIVE_WEBM,
  PlayerProtocol.WEBRTC,
])("preserves an encoded did:web in %s playback requests", async (protocol) => {
  await act(async () => {
    root.render(createElement(PlaybackURL, { src: DID, protocol }));
  });
  const url = new URL(container.textContent!);
  const requestedDID =
    protocol === PlayerProtocol.HLS
      ? url.searchParams.get("streamer")
      : decodeURIComponent(url.pathname.split("/")[3]);
  expect(requestedDID).toBe(DID);
});

it("preserves a did:web authority in VOD playlist requests", async () => {
  const uri = `at://${DID}/place.stream.video/3mvideo`;
  await act(async () => {
    root.render(
      createElement(PlaybackURL, { src: uri, protocol: PlayerProtocol.HLS }),
    );
  });
  expect(new URL(container.textContent!).searchParams.get("uri")).toBe(uri);
});
