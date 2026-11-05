import { act } from "react";
import { createRoot } from "react-dom/client";
import { afterEach, beforeEach, expect, it, vi } from "vitest";
import { makeLivestreamStore } from "../../core/src/livestream-store/store";
import { useLivestreamWebsocket } from "../src/livestream-provider/websocket";
import { LivestreamContext } from "../src/livestream-store/context";

const fixture = vi.hoisted(() => ({ options: {} as any }));
vi.mock("react-use-websocket", () => ({
  default: (_url: string, options: unknown) => {
    fixture.options = options;
  },
}));
vi.mock("../src/streamplace-store", () => ({
  useUrl: () => "https://node.example",
}));

let root: ReturnType<typeof createRoot>;
let store: ReturnType<typeof makeLivestreamStore>;
function Connection() {
  useLivestreamWebsocket("creator.example");
  return null;
}
beforeEach(async () => {
  vi.stubGlobal("IS_REACT_ACT_ENVIRONMENT", true);
  vi.useFakeTimers();
  store = makeLivestreamStore();
  store.setState({ presence: 3, websocketConnected: true });
  root = createRoot(document.createElement("div"));
  await act(async () =>
    root.render(
      <LivestreamContext.Provider value={{ store }}>
        <Connection />
      </LivestreamContext.Provider>,
    ),
  );
});
afterEach(async () => {
  await act(async () => root.unmount());
  expect(vi.getTimerCount()).toBe(0);
  vi.useRealTimers();
  vi.unstubAllGlobals();
});

it("clears presence when a connection opens or closes", async () => {
  await act(async () => fixture.options.onOpen());
  expect(store.getState().presence).toBeNull();
  expect(store.getState().websocketConnected).toBe(true);
  store.setState({ presence: 3 });
  await act(async () => fixture.options.onClose?.());
  expect(store.getState().presence).toBeNull();
  expect(store.getState().websocketConnected).toBe(false);
});

it("clears queued presence on disconnect without a delayed update restoring it", async () => {
  fixture.options.onMessage({
    data: JSON.stringify({
      $type: "place.stream.livestream#viewerCount",
      count: 7,
      presence: 5,
    }),
  });
  await act(async () => fixture.options.onClose?.());
  await act(async () => vi.advanceTimersByTime(250));
  expect(store.getState().presence).toBeNull();
  expect(store.getState().websocketConnected).toBe(false);
});

it.each(["delete", "hide"])(
  "applies a queued chat %s before disconnecting and keeps it removed on reconnect",
  async (removal) => {
    const message = {
      $type: "place.stream.chat.defs#messageView",
      uri: "at://did:plc:author/place.stream.chat.message/test",
      cid: "bafytest",
      author: { did: "did:plc:author", handle: "author.example" },
      indexedAt: new Date().toISOString(),
      record: {
        $type: "place.stream.chat.message",
        text: "A message that must be removed",
        createdAt: new Date().toISOString(),
      },
    };
    fixture.options.onMessage({ data: JSON.stringify(message) });
    await act(async () => vi.advanceTimersByTime(250));
    expect(store.getState().chat.map((chat) => chat.uri)).toEqual([
      message.uri,
    ]);

    fixture.options.onMessage({
      data: JSON.stringify(
        removal === "delete"
          ? { ...message, deleted: true }
          : {
              $type: "place.stream.chat.gate",
              hiddenMessage: message.uri,
              createdAt: new Date().toISOString(),
            },
      ),
    });
    fixture.options.onMessage({
      data: JSON.stringify({
        $type: "place.stream.livestream#viewerCount",
        count: 7,
        presence: 5,
      }),
    });
    await act(async () => fixture.options.onClose());
    expect(store.getState().chat).toEqual([]);
    expect(store.getState().presence).toBeNull();
    expect(store.getState().websocketConnected).toBe(false);
    expect(vi.getTimerCount()).toBe(0);

    await act(async () => fixture.options.onOpen());
    await act(async () => vi.advanceTimersByTime(250));
    expect(store.getState().chat).toEqual([]);
    expect(store.getState().presence).toBeNull();
    expect(store.getState().websocketConnected).toBe(true);
  },
);

it("cancels queued updates on unmount", async () => {
  fixture.options.onMessage({
    data: JSON.stringify({
      $type: "place.stream.livestream#viewerCount",
      count: 7,
      presence: 5,
    }),
  });
  await act(async () => root.unmount());
  expect(store.getState().presence).toBeNull();
  expect(vi.getTimerCount()).toBe(0);
});
