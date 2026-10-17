import { makeLivestreamStore } from "@streamplace/core";
import { act } from "react";
import { createRoot } from "react-dom/client";
import { expect, it, vi } from "vitest";
import { streamNotificationManager } from "../src/components/stream-notification/stream-notification-manager";
import { PinnedCommentWatcher } from "../src/livestream-provider";
import { LivestreamContext } from "../src/livestream-store/context";

vi.mock("../src/hooks", () => ({ useAvatars: () => ({}) }));
vi.mock("../src/streamplace-store", () => ({}));
vi.mock("../src/livestream-provider/websocket", () => ({}));
vi.mock("../src/livestream-store/hooks", () => ({
  useUnpinChatMessage: () => () => {},
}));
vi.mock(
  "../src/components/stream-notification",
  () =>
    import("../src/components/stream-notification/stream-notification-manager"),
);
vi.mock("../src/components/stream-notification/pin-notification", () => ({
  PinnedCommentNotification: () => null,
}));
vi.mock("../src/components/stream-notification/teleport-notification", () => ({
  TeleportNotification: () => null,
}));

it("keeps a new broadcast's pin when hiding the previous broadcast's notification", async () => {
  vi.stubGlobal("IS_REACT_ACT_ENVIRONMENT", true);
  const log = vi.spyOn(console, "log").mockImplementation(() => {});
  const store = makeLivestreamStore();
  const broadcast = (key: string) =>
    `at://did:plc:streamer/place.stream.livestream/${key}`;
  const livestream = (key: string) =>
    ({
      uri: broadcast(key),
      record: {},
    }) as never;
  const pin = (key: string) =>
    ({
      uri: `at://did:plc:streamer/place.stream.chat.pinnedRecord/${key}`,
      cid: key,
      record: { livestream: broadcast(key) },
    }) as never;
  const first = pin("first");
  const next = pin("next");
  store.setState({ livestream: livestream("first"), pinnedComment: first });
  const container = document.createElement("div");
  const root = createRoot(container);
  try {
    await act(async () =>
      root.render(
        <LivestreamContext.Provider value={{ store }}>
          <PinnedCommentWatcher />
        </LivestreamContext.Provider>,
      ),
    );
    expect(streamNotificationManager.getAll()).toHaveLength(1);

    await act(async () => {
      store.setState({ pinnedComment: next });
    });
    expect(streamNotificationManager.getAll()).toHaveLength(0);
    expect(store.getState().pinnedComment).toBe(next);

    await act(async () => {
      store.setState({ livestream: livestream("next") });
    });
    expect(streamNotificationManager.getAll()).toHaveLength(1);
    await act(async () => {
      streamNotificationManager.hide("pinned-comment");
    });
    expect(store.getState().pinnedComment).toBeNull();
  } finally {
    await act(async () => root.unmount());
    log.mockRestore();
    vi.unstubAllGlobals();
  }
});
