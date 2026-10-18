import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { connectLivestreamWebsocket } from "./connect";
import { makeLivestreamStore } from "./store";

type Listener = (event?: any) => void;

class FakeWebSocket {
  static instances: FakeWebSocket[] = [];
  url: string;
  onopen: Listener | null = null;
  onmessage: Listener | null = null;
  onclose: Listener | null = null;
  onerror: Listener | null = null;
  closed = false;

  constructor(url: string) {
    this.url = url;
    FakeWebSocket.instances.push(this);
  }
  close() {
    this.closed = true;
  }
}

let realWebSocket: typeof WebSocket;

beforeEach(() => {
  realWebSocket = globalThis.WebSocket;
  FakeWebSocket.instances = [];
  (globalThis as any).WebSocket = FakeWebSocket;
  vi.useFakeTimers();
});

afterEach(() => {
  (globalThis as any).WebSocket = realWebSocket;
  vi.useRealTimers();
});

describe("connectLivestreamWebsocket", () => {
  it("opens a socket at the given url", () => {
    const store = makeLivestreamStore();
    connectLivestreamWebsocket(store, "wss://example.com/socket");
    expect(FakeWebSocket.instances).toHaveLength(1);
    expect(FakeWebSocket.instances[0].url).toBe("wss://example.com/socket");
  });

  it("feeds messages through the reducer and marks the socket connected", async () => {
    const store = makeLivestreamStore();
    connectLivestreamWebsocket(store, "wss://example.com/socket");
    const ws = FakeWebSocket.instances[0];

    ws.onmessage!({
      data: JSON.stringify({
        $type: "place.stream.livestream#viewerCount",
        count: 7,
      }),
    });

    // flush happens on the next macrotask (setTimeout 0)
    await vi.advanceTimersByTimeAsync(1);
    expect(store.getState().viewers).toBe(7);
    expect(store.getState().websocketConnected).toBe(true);
  });

  it("does not flush a partial batch until the batch window elapses", async () => {
    const store = makeLivestreamStore();
    connectLivestreamWebsocket(store, "wss://example.com/socket", {
      batchWindowMs: 50,
    });
    const ws = FakeWebSocket.instances[0];

    ws.onmessage!({
      data: JSON.stringify({
        $type: "place.stream.livestream#viewerCount",
        count: 1,
      }),
    });
    expect(store.getState().viewers).toBeNull();

    await vi.advanceTimersByTimeAsync(50);
    expect(store.getState().viewers).toBe(1);
  });

  it("schedules a reconnect on close after the reconnect delay", async () => {
    const store = makeLivestreamStore();
    connectLivestreamWebsocket(store, "wss://example.com/socket");
    const ws = FakeWebSocket.instances[0];

    ws.onclose!();
    expect(store.getState().websocketConnected).toBe(false);
    expect(FakeWebSocket.instances).toHaveLength(1);

    await vi.advanceTimersByTimeAsync(3000);
    expect(FakeWebSocket.instances).toHaveLength(2);
  });

  it.each(["onclose", "onerror"] as const)(
    "drops teleport state and buffered messages on %s before reconnecting",
    async (event) => {
      const store = makeLivestreamStore();
      const handle = connectLivestreamWebsocket(
        store,
        "wss://example.com/socket",
        {
          batchWindowMs: 50,
        },
      );
      const ws = FakeWebSocket.instances[0];
      const teleport = {
        $type: "place.stream.live.teleport",
        uri: "at://did:plc:source/place.stream.live.teleport/1",
        cid: "teleport-cid",
        streamer: "did:plc:target",
        startsAt: new Date(Date.now() + 60_000).toISOString(),
      };
      ws.onmessage!({ data: JSON.stringify(teleport) });
      await vi.advanceTimersByTimeAsync(50);
      expect(store.getState().activeTeleportUri).toBe(teleport.uri);
      ws.onmessage!({ data: JSON.stringify(teleport) });

      ws[event]!();
      await vi.advanceTimersByTimeAsync(3_000);

      expect(store.getState().activeTeleport).toBeNull();
      expect(store.getState().activeTeleportUri).toBeNull();
      expect(store.getState().activeTeleportCID).toBeNull();
      expect(store.getState().websocketConnected).toBe(false);
      handle.disconnect();
    },
  );

  it.each([0, 50])(
    "resumes an imminent teleport across a drop with a %sms batch window",
    async (batchWindowMs) => {
      const store = makeLivestreamStore();
      const handle = connectLivestreamWebsocket(
        store,
        "wss://example.com/socket?view=chat",
        { batchWindowMs },
      );
      const teleport = {
        $type: "place.stream.live.teleport",
        uri: "at://did:plc:source/place.stream.live.teleport/1",
        cid: "teleport-cid",
        streamer: "did:plc:target",
        startsAt: new Date(Date.now() + 1_000).toISOString(),
      };
      FakeWebSocket.instances[0].onmessage!({ data: JSON.stringify(teleport) });
      if (batchWindowMs === 0) await vi.advanceTimersByTimeAsync(1);

      FakeWebSocket.instances[0].onclose!();
      await vi.advanceTimersByTimeAsync(3_000);
      const resumed = FakeWebSocket.instances[1];
      const url = new URL(resumed.url);
      expect(url.searchParams.get("teleportUri")).toBe(teleport.uri);
      expect(url.searchParams.get("teleportCid")).toBe(teleport.cid);
      expect(url.searchParams.get("view")).toBe("chat");
      expect(store.getState().activeTeleport).toBeNull();

      resumed.onmessage!({ data: JSON.stringify(teleport) });
      await vi.advanceTimersByTimeAsync(batchWindowMs + 1);
      expect(store.getState().activeTeleport).toEqual(teleport);
      handle.disconnect();
    },
  );

  it.each([0, 50])(
    "clears a canceled replacement's resume hint with a %sms batch window",
    async (batchWindowMs) => {
      const store = makeLivestreamStore();
      const handle = connectLivestreamWebsocket(
        store,
        "wss://example.com/socket",
        { batchWindowMs },
      );
      const teleport = {
        $type: "place.stream.live.teleport",
        uri: "at://did:plc:source/place.stream.live.teleport/1",
        cid: "teleport-cid",
        streamer: "did:plc:target",
        startsAt: new Date(Date.now() + 10_000).toISOString(),
      };
      FakeWebSocket.instances[0].onmessage!({ data: JSON.stringify(teleport) });
      await vi.advanceTimersByTimeAsync(batchWindowMs + 1);
      FakeWebSocket.instances[0].onclose!();
      await vi.advanceTimersByTimeAsync(3_000);

      const resumed = FakeWebSocket.instances[1];
      const replacement = {
        ...teleport,
        uri: teleport.uri + "2",
        cid: "replacement-cid",
      };
      resumed.onmessage!({ data: JSON.stringify([teleport, replacement]) });
      await vi.advanceTimersByTimeAsync(10_000);
      resumed.onmessage!({
        data: JSON.stringify({
          $type: "place.stream.livestream#teleportCanceled",
          teleportUri: replacement.uri,
          cid: replacement.cid,
        }),
      });
      if (batchWindowMs === 0) await vi.advanceTimersByTimeAsync(1);
      resumed.onclose!();
      await vi.advanceTimersByTimeAsync(3_000);

      const url = new URL(FakeWebSocket.instances[2].url);
      expect(url.searchParams.has("teleportUri")).toBe(false);
      expect(url.searchParams.has("teleportCid")).toBe(false);
      handle.disconnect();
    },
  );

  it("keeps the resume identity across reconnects that fail before hydration", async () => {
    const store = makeLivestreamStore();
    const handle = connectLivestreamWebsocket(
      store,
      "wss://example.com/socket",
    );
    store.setState({
      activeTeleportUri: "at://did:plc:source/place.stream.live.teleport/1",
      activeTeleportCID: "teleport-cid",
    });
    FakeWebSocket.instances[0].onclose!();
    await vi.advanceTimersByTimeAsync(3_000);
    FakeWebSocket.instances[1].onclose!();
    await vi.advanceTimersByTimeAsync(3_000);

    const url = new URL(FakeWebSocket.instances[2].url);
    expect(url.searchParams.get("teleportCid")).toBe("teleport-cid");
    handle.disconnect();
  });

  it("does not reconnect once disconnected", async () => {
    const store = makeLivestreamStore();
    const { disconnect } = connectLivestreamWebsocket(
      store,
      "wss://example.com/socket",
    );
    const ws = FakeWebSocket.instances[0];

    disconnect();
    ws.onmessage!({
      data: JSON.stringify({
        $type: "place.stream.livestream#viewerCount",
        count: 7,
      }),
    });
    ws.onclose!();
    await vi.advanceTimersByTimeAsync(3000);
    expect(FakeWebSocket.instances).toHaveLength(1);
    expect(store.getState().websocketConnected).toBe(false);
    expect(store.getState().viewers).toBeNull();
  });

  it("calls onDisconnect once per drop even when error triggers close", async () => {
    const onDisconnect = vi.fn();
    const store = makeLivestreamStore();
    connectLivestreamWebsocket(store, "wss://example.com/socket", {
      reconnectDelayMs: 1000,
      onDisconnect,
    });
    const ws1 = FakeWebSocket.instances[0];

    // onerror closes the socket, which fires onclose async; the reconnect
    // guard must prevent a second onDisconnect / second timer.
    ws1.onerror!();
    ws1.onclose!();
    expect(onDisconnect).toHaveBeenCalledTimes(1);

    // reconnect opens a fresh socket
    await vi.advanceTimersByTimeAsync(1000);
    expect(FakeWebSocket.instances).toHaveLength(2);

    // a second drop on the new socket fires onDisconnect again
    FakeWebSocket.instances[1].onclose!();
    expect(onDisconnect).toHaveBeenCalledTimes(2);
  });
});
