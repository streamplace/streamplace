import {
  handleWebSocketMessages,
  makeLivestreamStore,
} from "@streamplace/core";
import { act, createElement, useState, type ReactNode } from "react";
import { createRoot } from "react-dom/client";
import { afterEach, beforeAll, beforeEach, expect, it, vi } from "vitest";
import InformationWidget from "../src/components/dashboard/information-widget";
import { LivestreamContext } from "../src/livestream-store/context";

const warnings = vi.hoisted(() =>
  vi.spyOn(console, "warn").mockImplementation(() => {}),
);
beforeAll(() => {
  for (const [message] of warnings.mock.calls)
    expect(message).toBe(
      "[@atproto/lex-data]: Uint8Array.fromBase64 / Uint8Array.prototype.toBase64 not available in this environment. Falling back to ponyfill implementation.",
    );
  warnings.mockRestore();
});
// Platform drawing bindings only. Widget logic, metric tiles, history, and SVG
// path construction are real; no sample or status calculation is mocked.
vi.mock("react-native", () => {
  const View = ({ children }: { children: ReactNode }) =>
    createElement("div", null, children);
  return {
    View,
    Text: View,
    Pressable: View,
    TouchableOpacity: View,
    Platform: { OS: "web" },
  };
});
vi.mock("react-native-svg", () => ({
  default: "svg",
  Path: "path",
  Line: "line",
  Text: "text",
}));
vi.mock("lucide-react-native", () =>
  Object.fromEntries(
    [
      "Car",
      "ChevronDown",
      "ChevronUp",
      "Eye",
      "EyeClosed",
      "Monitor",
      "Signal",
      "Video",
      "Zap",
    ].map((name) => [name, () => null]),
  ),
);
vi.mock("../src/hooks", () => ({
  useAQState: (_key: string, initial: boolean) => useState(initial),
}));
vi.mock(
  "../src/livestream-store",
  () => import("../src/livestream-store/use-store"),
);
vi.mock("../src/lib/theme/theme", () => ({
  useTheme: () => ({ theme: { colors: {} } }),
}));
vi.mock("../src/components/ui", async () => ({
  ...(await import("../src/components/ui/info-box")),
  ...(await import("../src/components/ui/info-row")),
  Badge: ({ children }: { children: ReactNode }) => children,
}));
vi.mock("react-i18next", () => ({
  useTranslation: () => ({
    t: (key: string) =>
      key === "creator-readings-stale" ? "Readings stale" : key,
  }),
}));

let store = makeLivestreamStore();
let root: ReturnType<typeof createRoot>;
let container: HTMLDivElement;
beforeEach(() => {
  vi.stubGlobal("IS_REACT_ACT_ENVIRONMENT", true);
  store = makeLivestreamStore();
  container = document.createElement("div");
  root = createRoot(container);
});
afterEach(async () => {
  await act(async () => root.unmount());
  vi.unstubAllGlobals();
});
async function render(readingsStale = false, wideMode = true) {
  await act(async () =>
    root.render(
      <LivestreamContext.Provider value={{ store }}>
        <InformationWidget
          wideMode={wideMode}
          readingsStale={readingsStale}
          connectionQuality="pre-live"
        />
      </LivestreamContext.Provider>,
    ),
  );
}
function receiveSegment() {
  store.setState((state) =>
    handleWebSocketMessages(state, [
      {
        $type: "place.stream.segment",
        startTime: new Date().toISOString(),
        duration: 1_000_000_000,
        size: 250_000,
        video: [{ width: 1920, height: 1080, framerate: { num: 30, den: 1 } }],
      },
    ]),
  );
}

it("keeps absent metrics and viewer counts unavailable rather than measured zero", async () => {
  await render(true);
  expect(container.textContent).not.toContain("0.00 Mbps");
  expect(container.textContent).not.toMatch(/\b0 viewers\b/);
  expect(container.textContent).not.toContain("Readings stale");
  expect(container.textContent).toContain("—");
  expect(container.querySelectorAll("path[d]")).toHaveLength(0);
});

it("renders valid readings without fabricating a history or an invalid single-point path", async () => {
  receiveSegment();
  await render();
  expect(container.textContent).toContain("2.00 Mbps");
  expect(container.textContent).toContain("1920x1080");
  expect(container.textContent).toContain("30.00 FPS");
  expect(container.textContent).toContain("AVG2.0M");
  expect(
    container.querySelector('path[d*="NaN"], path[d*="Infinity"]'),
  ).toBeNull();
});

it("records unchanged bitrate from a second real segment", async () => {
  receiveSegment();
  await render();
  expect(container.querySelectorAll("path[d]")).toHaveLength(0);
  await act(async () => receiveSegment());
  expect(container.querySelectorAll("path[d]")).toHaveLength(1);
  expect(
    container.querySelector('path[d*="NaN"], path[d*="Infinity"]'),
  ).toBeNull();
});

it("keeps pre-announcement connection quality unavailable instead of poor", async () => {
  receiveSegment();
  await render(false, false);
  expect(container.textContent).toContain("Connection—");
  expect(container.textContent).not.toContain("Poor");
});

it("labels retained readings stale and does not append a sample on disconnection", async () => {
  receiveSegment();
  await render();
  const paths = [...container.querySelectorAll("path[d]")].map((path) =>
    path.getAttribute("d"),
  );
  await render(true);
  expect(container.textContent).toContain("Readings stale");
  expect(container.textContent).toContain("2.00 Mbps");
  expect(
    [...container.querySelectorAll("path[d]")].map((path) =>
      path.getAttribute("d"),
    ),
  ).toEqual(paths);
});
