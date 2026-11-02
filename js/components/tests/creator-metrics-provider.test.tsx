import {
  handleWebSocketMessages,
  makeLivestreamStore,
} from "@streamplace/core";
import { act, type ReactNode } from "react";
import { createRoot } from "react-dom/client";
import { afterEach, beforeAll, expect, it, vi } from "vitest";
import CreatorMetricsProvider, {
  useCreatorMetrics,
} from "../../app/components/live-dashboard/creator-metrics-provider.web";
import { LivestreamContext } from "../src/livestream-store/context";

const warnings = vi.hoisted(() =>
  vi.spyOn(console, "warn").mockImplementation(() => {}),
);
const timing = vi.hoisted(() => ({ measured: false }));
beforeAll(() => {
  for (const [message] of warnings.mock.calls) {
    expect(message).toBe(
      "[@atproto/lex-data]: Uint8Array.fromBase64 / Uint8Array.prototype.toBase64 not available in this environment. Falling back to ponyfill implementation.",
    );
  }
  warnings.mockRestore();
});
vi.mock("@streamplace/components", async () => ({
  getStoreFromContext: (await import("../src/livestream-store/use-store"))
    .getStoreFromContext,
}));
// Segment timing is a clock-dependent input. Sampling and both stores are real.
vi.mock("hooks/useLiveDashboardStatus", () => ({
  useLiveDashboardStatus: () => ({
    timeBetweenSegments: 1000,
    hasTimingReadings: timing.measured,
  }),
}));

let root: ReturnType<typeof createRoot> | undefined;
afterEach(async () => {
  if (root) await act(async () => root?.unmount());
  root = undefined;
  vi.useRealTimers();
  vi.unstubAllGlobals();
});

it("keeps samples across creator child changes and resets them with the account store", async () => {
  vi.stubGlobal("IS_REACT_ACT_ENVIRONMENT", true);
  vi.useFakeTimers();
  const now = Date.parse("2026-10-26T12:00:00Z");
  vi.setSystemTime(now);
  const first = makeLivestreamStore();
  first.setState((state) =>
    handleWebSocketMessages(state, [
      {
        $type: "place.stream.segment",
        startTime: new Date(now).toISOString(),
        duration: 1_000_000_000,
        size: 250_000,
      },
    ] as any),
  );
  const second = makeLivestreamStore();
  let current!: ReturnType<typeof useCreatorMetrics>;
  function Section({ children }: { children: ReactNode }) {
    current = useCreatorMetrics();
    return <span>{children}</span>;
  }
  const element = document.createElement("div");
  root = createRoot(element);
  const render = async (store: typeof first, key: string, label: string) => {
    await act(async () =>
      root!.render(
        <LivestreamContext.Provider key={key} value={{ store }}>
          <CreatorMetricsProvider>
            <Section>{label}</Section>
          </CreatorMetricsProvider>
        </LivestreamContext.Provider>,
      ),
    );
  };

  await render(first, "first", "Stream");
  expect(current.history).toEqual([]);
  await act(async () => vi.advanceTimersByTime(2000));
  expect(current.history).toHaveLength(1);
  expect(current.history[0].bitrateKbps).toBe(2000);
  expect(current.history[0].segmentMs).toBeNull();
  timing.measured = true;
  await render(first, "first", "Chat");
  await act(async () => vi.advanceTimersByTime(2000));
  expect(current.history).toHaveLength(2);
  expect(current.history[1].segmentMs).toBe(1000);
  await act(async () => current.setRange("1m"));
  await act(async () => current.setRange("25m"));
  expect(current.history).toHaveLength(2);

  await render(second, "second", "Stream");
  expect(current.history).toEqual([]);
  await act(async () => vi.advanceTimersByTime(2000));
  expect(current.history).toHaveLength(1);
  expect(current.history[0].bitrateKbps).toBeNull();
});
