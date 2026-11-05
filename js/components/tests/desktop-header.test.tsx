import { act, createElement } from "react";
import { createRoot } from "react-dom/client";
import { afterEach, beforeEach, expect, it, vi } from "vitest";
import { TopControlBar } from "../../app/components/mobile/desktop-ui/top-controls";

const fixture = vi.hoisted(() => ({ fullscreen: false }));
function View({ children }: any) {
  return createElement("div", null, children);
}

// Render native presentation as DOM; header identity and fullscreen selection
// use the real component and handle formatting.
vi.mock("react-native", () => ({
  View,
  Pressable: View,
  Platform: { OS: "web" },
  Linking: { openURL() {} },
  useWindowDimensions: () => ({ width: 1280, height: 720 }),
}));
vi.mock("@react-navigation/native", () => ({ useNavigation: () => ({}) }));
vi.mock("@streamplace/components", async () => ({
  View,
  Text: View,
  ContentWarningBadge: () => null,
  PlayerUI: { Viewers: () => null },
  zero: await import("../src/lib/theme/atoms"),
  useAuthor: () => ({ did: "did:plc:creator", handle: "creator.example" }),
  useAvatar: () => "https://creator.example/avatar.jpg",
  useTitle: () => "A creator's video",
  useCameraToggle: () => ({}),
  useLivestreamStore: () => null,
  usePlayerStore: (select: any) => select(fixture),
  useTheme: () => ({ theme: { colors: {} } }),
}));
vi.mock("@streamplace/core", () => import("../../core/src/lib/format-handle"));
vi.mock("components/brand/logo", () => ({ LogoTile: () => null }));
vi.mock("../../app/src/navigation-helper", () => ({}));
vi.mock("../../app/components/mobile/desktop-ui/live-bubble", () => ({
  LiveBubble: () => null,
}));
vi.mock("expo-image", () => ({ Image: () => null }));
vi.mock("lucide-react-native", () => ({
  ChevronLeft: () => null,
  MessageSquare: () => null,
  SwitchCamera: () => null,
}));

let container: HTMLDivElement;
let root: ReturnType<typeof createRoot>;
beforeEach(() => {
  vi.stubGlobal("IS_REACT_ACT_ENVIRONMENT", true);
  fixture.fullscreen = false;
  container = document.createElement("div");
  root = createRoot(container);
});
afterEach(async () => {
  await act(async () => root.unmount());
  vi.unstubAllGlobals();
});

async function render(embedded = false) {
  await act(async () =>
    root.render(
      <TopControlBar
        embedded={embedded}
        offline={false}
        isActivelyLive={false}
        ingest={null}
        isChatOpen={false}
        onToggleChat={() => {}}
      />,
    ),
  );
}

it("adds creator context in fullscreen and removes duplication when returning", async () => {
  await render();
  expect(container.textContent).not.toContain("A creator's video");
  fixture.fullscreen = true;
  await render();
  expect(container.textContent).toContain("A creator's video");
  expect(container.textContent).toContain("@creator.example");
  fixture.fullscreen = false;
  await render();
  expect(container.textContent).not.toContain("creator.example");
});

it("keeps creator identity in embedded playback", async () => {
  await render(true);
  expect(container.textContent).toContain("A creator's video");
  expect(container.textContent).toContain("@creator.example");
});
