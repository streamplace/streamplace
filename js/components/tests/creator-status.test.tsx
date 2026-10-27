import {
  handleWebSocketMessages,
  makeLivestreamStore,
} from "@streamplace/core";
import { act } from "react";
import { createRoot } from "react-dom/client";
import { afterEach, beforeAll, beforeEach, expect, it, vi } from "vitest";
import { useLiveDashboardStatus } from "../../app/hooks/useLiveDashboardStatus";
import { useLivestreamWebsocket } from "../src/livestream-provider/websocket";
import { LivestreamContext } from "../src/livestream-store/context";

const transport = vi.hoisted(() => ({
  options: {} as any,
  warnings: vi.spyOn(console, "warn").mockImplementation(() => {}),
}));
beforeAll(() => {
  for (const [message] of transport.warnings.mock.calls) {
    expect(message).toBe(
      "[@atproto/lex-data]: Uint8Array.fromBase64 / Uint8Array.prototype.toBase64 not available in this environment. Falling back to ponyfill implementation.",
    );
  }
  transport.warnings.mockRestore();
});
vi.mock(
  "../src/livestream-store",
  () => import("../src/livestream-store/use-store"),
);
vi.mock("@streamplace/components", async () => ({
  ...(await import("../src/livestream-store/use-store")),
  ...(await import("../src/hooks/useSegmentTiming")),
}));
vi.mock("../src/streamplace-store", () => ({
  useUrl: () => "https://node.example",
}));
// The transport callback boundary is controlled; buffering, reducer updates,
// connection loss and React cleanup run through the production hook.
vi.mock("react-use-websocket", () => ({
  default: (_url: string, options: unknown) => {
    transport.options = options;
  },
}));

let store = makeLivestreamStore();
let root: ReturnType<typeof createRoot>;
let status: ReturnType<typeof useLiveDashboardStatus>;

function Status() {
  status = useLiveDashboardStatus();
  return null;
}
function Socket() {
  useLivestreamWebsocket("did:plc:creator");
  return null;
}

beforeEach(() => {
  vi.stubGlobal("IS_REACT_ACT_ENVIRONMENT", true);
  vi.useFakeTimers();
  vi.setSystemTime(new Date("2026-10-26T12:00:00Z"));
  store = makeLivestreamStore();
  root = createRoot(document.createElement("div"));
});
afterEach(async () => {
  await act(async () => root.unmount());
  vi.useRealTimers();
  vi.unstubAllGlobals();
});

async function render(child: React.ReactNode) {
  await act(async () =>
    root.render(
      <LivestreamContext.Provider value={{ store }}>
        {child}
      </LivestreamContext.Provider>,
    ),
  );
}
function segment() {
  return {
    $type: "place.stream.segment",
    startTime: new Date().toISOString(),
    duration: 1_000_000_000,
    size: 250_000,
  };
}
async function emit() {
  await act(async () =>
    store.setState((state) => handleWebSocketMessages(state, [segment()])),
  );
}

it("ages a live reading without another store event and releases its clock", async () => {
  store.setState({ websocketConnected: true });
  await render(<Status />);
  expect(status.phase).toBe("waiting");
  await emit();
  expect(status.phase).toBe("ready");
  await act(async () => vi.advanceTimersByTime(9999));
  expect(status.incoming).toBe(true);
  await act(async () => vi.advanceTimersByTime(1001));
  expect(status).toMatchObject({
    phase: "waiting",
    readingsStale: true,
    canAnnounce: false,
  });
  expect(store.getState().segment).not.toBeNull();
  await act(async () => root.unmount());
  expect(vi.getTimerCount()).toBe(0);
});

it("degrades segment quality when arrivals stall", async () => {
  store.setState({ livestream: { record: { title: "Live" } } as any });
  await render(<Status />);
  await emit();
  await act(async () => vi.advanceTimersByTime(1000));
  await emit();
  expect(status.connectionQuality).toBe("good");
  await act(async () => vi.advanceTimersByTime(4000));
  expect(status.connectionQuality).toBe("poor");
});

it("recognizes a fresh segment arriving between clock ticks", async () => {
  store.setState({ websocketConnected: true });
  await render(<Status />);
  await act(async () => vi.advanceTimersByTime(500));
  await emit();
  expect(status).toMatchObject({ phase: "ready", incoming: true });
});

it.each(["onClose", "onError"])(
  "marks a socket %s stale and drops its pending batch",
  async (event) => {
    store.setState({ websocketConnected: true });
    await render(<Socket />);
    transport.options.onMessage({ data: JSON.stringify(segment()) });
    await act(async () => transport.options[event]({}));
    await act(async () => vi.advanceTimersByTime(300));
    expect(store.getState().websocketConnected).toBe(false);
    expect(store.getState().segment).toBeNull();
  },
);

it("cancels a queued message batch when the stream session unmounts", async () => {
  await render(<Socket />);
  transport.options.onMessage({ data: JSON.stringify(segment()) });
  await act(async () => root.unmount());
  await act(async () => vi.advanceTimersByTime(300));
  expect(store.getState().segment).toBeNull();
  expect(vi.getTimerCount()).toBe(0);
});
