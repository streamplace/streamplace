import { act, createElement } from "react";
import { createRoot } from "react-dom/client";
import { afterEach, beforeEach, expect, it, vi } from "vitest";
import { useStore } from "zustand";
import { LiveStats } from "../../app/components/mobile/live-stats";
import { makeLivestreamStore } from "../../core/src/livestream-store/store";
import { spacing, typeScale } from "../src/lib/theme/tokens";

const fixture = vi.hoisted(() => ({
  store: null as ReturnType<typeof makeLivestreamStore> | null,
  mode: "live",
}));
function View({ children, accessibilityLabel, style }: any) {
  return (
    <div
      aria-label={accessibilityLabel}
      style={Object.assign({}, ...[style].flat(Infinity))}
    >
      {children}
    </div>
  );
}
function Text({ children, accessibilityLabel }: any) {
  return <span aria-label={accessibilityLabel}>{children}</span>;
}
vi.mock("react-native", () => ({
  View,
  Text,
  StyleSheet: { create: (styles: unknown) => styles },
  Platform: { OS: "web", select: () => null },
}));
vi.mock("lucide-react-native", () => ({ UsersRound: () => null }));
vi.mock("@streamplace/components", async () => {
  const atoms = await import("../src/lib/theme/atoms");
  return {
    Text,
    View,
    zero: atoms,
    useLivestreamStore: (select: any) => useStore(fixture.store!, select),
    usePlayerStore: (select: any) => select({ mode: fixture.mode }),
    useViews: () => useStore(fixture.store!, (state) => state.viewers),
    useTranslation: () => ({
      t: (_key: string, values: any) => `${values.count} watching`,
    }),
  };
});
vi.mock("../src/components/ui", () => ({
  useTheme: () => ({ theme: { colors: { live: "red" }, spacing, typeScale } }),
}));
vi.mock("../src/components/ui/text", () => ({ Text }));

let root: ReturnType<typeof createRoot>;
let container: HTMLDivElement;
beforeEach(() => {
  vi.stubGlobal("IS_REACT_ACT_ENVIRONMENT", true);
  vi.useFakeTimers();
  vi.setSystemTime(new Date("2026-10-30T12:00:00Z"));
  fixture.mode = "live";
  fixture.store = makeLivestreamStore();
  fixture.store.setState({
    viewers: 12,
    livestream: {
      record: {
        createdAt: "2026-10-30T10:46:45Z",
        lastSeenAt: new Date().toISOString(),
      },
    } as any,
    segment: { startTime: new Date().toISOString() } as any,
  });
  container = document.createElement("div");
  root = createRoot(container);
});
afterEach(async () => {
  await act(async () => root.unmount());
  expect(vi.getTimerCount()).toBe(0);
  vi.useRealTimers();
  vi.unstubAllGlobals();
});
async function render() {
  await act(async () => root.render(createElement(LiveStats)));
}

it("shows only the viewer count and keeps it current", async () => {
  await render();
  expect(container.textContent).toBe("12");
  expect(container.querySelector('[aria-label="12 watching"]')).not.toBeNull();
  await act(async () => {
    vi.advanceTimersByTime(1000);
    fixture.store!.setState({ viewers: 24 });
  });
  expect(container.textContent).toBe("24");
  expect(container.querySelector('[aria-label="24 watching"]')).not.toBeNull();
});

it.each([0, null])(
  "preserves zero viewers and hides an unknown count (%s)",
  async (viewers) => {
    fixture.store!.setState({ viewers });
    await render();
    expect(container.textContent).toBe(viewers === 0 ? "0" : "");
    expect(container.querySelector('[aria-label="0 watching"]') !== null).toBe(
      viewers === 0,
    );
  },
);

it.each(["offline", "ended", "vod"])(
  "hides the live readout when %s",
  async (state) => {
    if (state === "vod") fixture.mode = "vod";
    const livestream = fixture.store!.getState().livestream!;
    if (state === "ended")
      livestream.record.endedAt = new Date().toISOString() as any;
    if (state === "offline") {
      livestream.record.lastSeenAt = "2026-10-30T10:00:00Z" as any;
      fixture.store!.setState({ segment: null });
    }
    await render();
    expect(container.textContent).toBe("");
  },
);

it("hides the viewer count when live activity expires", async () => {
  await render();
  expect(container.textContent).toBe("12");
  await act(async () => vi.advanceTimersByTime(10_000));
  expect(container.textContent).toBe("");
});
