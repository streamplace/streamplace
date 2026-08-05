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

  it("does not reconnect once disconnected", async () => {
    const store = makeLivestreamStore();
    const { disconnect } = connectLivestreamWebsocket(
      store,
      "wss://example.com/socket",
    );
    const ws = FakeWebSocket.instances[0];

    disconnect();
    ws.onclose!();
    await vi.advanceTimersByTimeAsync(3000);
    expect(FakeWebSocket.instances).toHaveLength(1);
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
