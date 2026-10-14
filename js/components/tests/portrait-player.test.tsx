import type { ReactNode } from "react";
import { act, createElement } from "react";
import { createRoot } from "react-dom/client";
import { afterEach, beforeEach, expect, it, vi } from "vitest";
import { PlayerInner } from "../../app/components/mobile/player";
import { Fullscreen } from "../src/components/mobile-player/fullscreen.native";
import { streamNotificationManager } from "../src/components/stream-notification/stream-notification-manager";

const fixture = vi.hoisted(() => ({
  window: { width: 390, height: 844 },
  video: { width: 1080, height: 1920 },
  insets: { top: 47, bottom: 34, left: 0, right: 0 },
  fullscreen: false,
  platform: "ios",
  profile: { did: "did:plc:streamer", handle: "streamer.example" } as {
    did: string;
    handle: string;
  } | null,
  warnings: ["nudity"],
  openURL: vi.fn(),
}));

// Only presentation bindings and external screen/stream inputs are replaced.
// Control selection, fullscreen sizing, and notification dismissal run unchanged.
function View({
  children,
  style,
  edges,
  pointerEvents,
}: {
  children?: ReactNode;
  style?: any;
  edges?: string[];
  pointerEvents?: string;
}) {
  const flattenedStyle = Object.assign({}, ...[style].flat(Infinity));
  return createElement(
    "div",
    {
      "data-style": JSON.stringify(flattenedStyle),
      "data-edges": edges?.join(","),
      "data-pointer-events": pointerEvents ?? flattenedStyle.pointerEvents,
    },
    children,
  );
}
vi.mock("react-native", () => ({
  View,
  Pressable: ({ children, onPress, accessibilityLabel, style }: any) =>
    createElement(
      "button",
      {
        onClick: onPress,
        "aria-label": accessibilityLabel,
        "data-style": JSON.stringify(style),
      },
      children,
    ),
  Linking: { openURL: fixture.openURL },
  ScrollView: View,
  StatusBar: () => null,
  Platform: {
    get OS() {
      return fixture.platform;
    },
    select: () => null,
  },
  StyleSheet: { create: (styles: unknown) => styles },
  Dimensions: {
    get: () => fixture.window,
  },
  useWindowDimensions: () => fixture.window,
  BackHandler: { addEventListener: () => ({ remove() {} }) },
}));
vi.mock("react-native-reanimated", () => ({
  default: { View, createAnimatedComponent: () => View },
  useSharedValue: (value: unknown) => ({ value }),
  useAnimatedStyle: (style: () => unknown) => style(),
  withTiming: (value: unknown) => value,
  withSpring: (value: unknown) => value,
  runOnJS: (callback: unknown) => callback,
  interpolate: () => 1,
  Extrapolation: { CLAMP: "clamp" },
  Easing: {
    bezier() {},
    out() {},
    in() {},
    cubic() {},
  },
}));
vi.mock("react-native-safe-area-context", () => ({
  useSafeAreaInsets: () => fixture.insets,
  SafeAreaView: View,
}));
vi.mock("react-native-gesture-handler", () => {
  const gesture = {
    onChange() {
      return this;
    },
    onEnd() {
      return this;
    },
    onStart() {
      return this;
    },
    onUpdate() {
      return this;
    },
  };
  return {
    Gesture: {
      Hover: () => gesture,
      Pan: () => gesture,
      Tap: () => gesture,
      Race() {},
    },
    GestureDetector: View,
    Pressable: View,
  };
});
vi.mock("expo-image", () => ({ Image: View }));
vi.mock("../src/components/ui/view", () => ({ View }));
vi.mock("../src/hooks", () => ({
  useKeyboardSlide: () => ({ slideKeyboard: 0 }),
}));
vi.mock("hooks/useKeyboard", () => ({
  useKeyboard: () => ({ keyboardHeight: 0 }),
}));
vi.mock("@streamplace/components/src/streamplace-store/xrpc", () => ({
  usePDSAgent: () => undefined,
}));
vi.mock("components/emoji-picker/emoji-picker", () => ({ EmojiPicker: View }));
vi.mock("components/mobile/badge-picker", () => ({ BadgePicker: View }));
vi.mock("utils/emoji", () => ({ useEmojiData: () => ({}) }));
vi.mock(
  "@streamplace/components/src/ui",
  () => import("../src/lib/theme/atoms"),
);
vi.mock("../../app/components/mobile/desktop-ui/index", () => ({
  BottomControlBar: () => null,
}));
vi.mock("react-native-edge-to-edge", () => ({
  SystemBars: { setHidden() {} },
}));
vi.mock("expo-video", () => ({ VideoView: View }));
vi.mock("lucide-react-native", () => ({
  ArrowLeft: View,
  ChevronLeft: View,
  ChevronRight: View,
  ChevronUp: View,
  ChevronsRight: View,
  Maximize: View,
  Minimize: View,
}));
vi.mock("@react-navigation/native", () => ({ useNavigation: () => ({}) }));
vi.mock("hooks/useLiveUser", () => ({ useLiveUser: () => false }));
vi.mock("hooks/useSidebarControl", () => ({
  useSidebarControl: () => ({ contentMargin: 0, isActive: false }),
}));
vi.mock("store/hooks", () => ({ useUserProfile: () => null }));
vi.mock("store", () => ({ useStore: () => () => {} }));
vi.mock("../../app/src/navigation-helper", () => ({
  convertNavigationParams() {},
}));
vi.mock("../../app/components/mobile/bottom-metadata", () => ({
  BottomMetadata: View,
}));
vi.mock("../../app/components/mobile/chat", () => ({
  DesktopChatPanel: View,
  MobileChatPanel: View,
}));
vi.mock("../../app/components/mobile/desktop-ui", () => ({
  DesktopUi: () => <button>Desktop controls</button>,
}));
vi.mock("../../app/components/mobile/offline-counter", () => ({
  OfflineCounter: () => null,
}));
vi.mock("../../app/components/mobile/user-offline", () => ({
  UserOffline: View,
}));
vi.mock("../../app/components/mobile/ui", () => ({
  MobileUi: () => <button>Mobile controls and chat</button>,
}));
vi.mock("../src/components/mobile-player/video.native", () => ({
  default: () => <span>Video</span>,
}));
vi.mock("../src/components/mobile-player/ui/audio-only-overlay", () => ({
  AudioOnlyOverlay: () => null,
}));

async function bindings() {
  const atoms = await import("../src/lib/theme/atoms");
  const tokens = await import("../src/lib/theme/tokens");
  return {
    View,
    zero: atoms,
    useTheme: () => ({ theme: tokens }),
    Player: View,
    PlayerUI: {
      ViewerLoadingOverlay: () => null,
      ContextMenu: () => <span>Menu</span>,
      AutoplayButton: () => null,
      CountdownOverlay: () => null,
      LoadingOverlay: () => null,
    },
    VideoRetry: View,
    VodSection: View,
    PlayerProtocol: { WEBRTC: "webrtc" },
    usePlayerStore: (select: (state: any) => unknown) =>
      select({
        protocol: "webrtc",
        fullscreen: fixture.fullscreen,
        setFullscreen() {},
        setSrc() {},
        mode: "live",
        status: "playing",
        selectedRendition: "audio",
      }),
    usePlayerDimensions: () => fixture.window,
    useSegmentDimensions: () => fixture.video,
    useLivestreamStore: (select?: (state: any) => unknown) =>
      select?.({
        segment: { contentWarnings: { warnings: fixture.warnings } },
      }),
    PlayerStatus: { PLAYING: "playing" },
    useLivestreamInfo: () => ({ ingest: null, profile: fixture.profile }),
    useAuthor: () => fixture.profile,
    useAvatar: () => undefined,
    useCameraToggle: () => ({}),
    useLivestream: () => null,
    useMuted: () => false,
    useSetMuted: () => () => {},
    useRotation: () => ({ canRotate: false }),
    useNetworkProfileUrl: () => () => "https://network.example/streamer",
    Text: ({ children }: { children: ReactNode }) => <span>{children}</span>,
    Avatar: () => <span>Avatar</span>,
    ShareSheet: () => <span>Share</span>,
    Toast: () => null,
    ContentWarningBadge: () => <span>Intended for certain audiences</span>,
    useDanmuEnabled: () => false,
    useDanmuOpacity: () => 1,
    useDanmuSpeed: () => 1,
    useDanmuLaneCount: () => 1,
    useDanmuMaxMessages: () => 1,
    DanmuOverlay: () => null,
    Chat: () => null,
    Loader: () => null,
    IconButton: View,
  };
}
vi.mock("@streamplace/components", async () => {
  const { Resizable } = await import("../src/components/ui/resizeable");
  const { StreamNotificationProvider } =
    await import("../src/components/stream-notification/stream-notification");
  return {
    ...(await bindings()),
    Resizable,
    StreamNotificationProvider,
  };
});
vi.mock("../src", bindings);

let container: HTMLDivElement;
let root: ReturnType<typeof createRoot>;
beforeEach(() => {
  vi.stubGlobal("IS_REACT_ACT_ENVIRONMENT", true);
  fixture.window = { width: 390, height: 844 };
  fixture.video = { width: 1080, height: 1920 };
  fixture.insets = { top: 47, bottom: 34, left: 0, right: 0 };
  fixture.fullscreen = false;
  fixture.platform = "ios";
  fixture.profile = { did: "did:plc:streamer", handle: "streamer.example" };
  fixture.warnings = ["nudity"];
  fixture.openURL.mockClear();
  container = document.createElement("div");
  root = createRoot(container);
});

it.each(["ios", "web"])(
  "puts the streamer profile above the audience notice on %s",
  async (platform) => {
    fixture.platform = platform;
    const { MobileUi } = await vi.importActual<
      typeof import("../../app/components/mobile/ui")
    >("../../app/components/mobile/ui");
    await act(async () => root.render(<MobileUi hideMobileChat />));
    const profile = container.querySelector<HTMLButtonElement>(
      '[aria-label="Open profile for streamer.example"]',
    );
    expect(profile).not.toBeNull();
    expect(profile!.textContent).toContain("streamer.example");
    expect(profile!.parentElement!.textContent).toContain("Share");
    expect(profile!.parentElement!.textContent).not.toContain(
      "Intended for certain audiences",
    );
    const top = profile!.closest('[data-edges="top"]')!;
    expect(top.textContent).toContain("Intended for certain audiences");
    await act(async () => profile!.click());
    expect(fixture.openURL).toHaveBeenCalledWith(
      "https://network.example/streamer",
    );
  },
);
afterEach(async () => {
  await act(async () => root.unmount());
  if (streamNotificationManager.getAll().length)
    streamNotificationManager.hide("pinned-comment");
  vi.restoreAllMocks();
  vi.useRealTimers();
  vi.unstubAllGlobals();
});

it.each(["portrait", "landscape"])(
  "allows dismissing a pinned comment through the %s chat overlay",
  async (orientation) => {
    vi.useFakeTimers();
    vi.spyOn(console, "log").mockImplementation(() => {});
    const { MobileChatPanel, DesktopChatPanel } = await vi.importActual<
      typeof import("../../app/components/mobile/chat")
    >("../../app/components/mobile/chat");
    streamNotificationManager.show({
      id: "pinned-comment",
      duration: 0,
      render: (_isExiting, dismiss) => (
        <button onClick={() => dismiss("user")}>Dismiss pinned comment</button>
      ),
    });
    await act(async () =>
      root.render(
        orientation === "portrait" ? (
          <MobileChatPanel isPlayerRatioGreater={false} />
        ) : (
          <DesktopChatPanel
            chatVisible
            chatPanelWidth={350}
            setShowChat={() => {}}
          />
        ),
      ),
    );
    const dismiss = container.querySelector<HTMLButtonElement>("button")!;
    expect(dismiss.textContent).toBe("Dismiss pinned comment");
    // A native parent with pointerEvents=none disables every child, even one
    // that explicitly enables its own pointer events.
    for (
      let parent = dismiss.parentElement;
      parent;
      parent = parent.parentElement
    ) {
      expect(parent.dataset.pointerEvents).not.toBe("none");
    }
    await act(async () => dismiss.click());
    await act(async () => vi.advanceTimersByTime(200));
    expect(streamNotificationManager.getAll()).toHaveLength(0);
    expect(container.textContent).not.toContain("Dismiss pinned comment");
  },
);

it.each(["ios", "android", "web"])(
  "renders mobile controls for a portrait live stream on %s",
  async (platform) => {
    fixture.platform = platform;
    await act(async () =>
      root.render(
        <PlayerInner showChat setShowChat={() => {}} showUnavailable={false} />,
      ),
    );
    expect(container.textContent).toContain("Mobile controls and chat");
  },
);

it("keeps the detached landscape controls from appearing twice", async () => {
  await act(async () =>
    root.render(
      <PlayerInner
        showChat
        setShowChat={() => {}}
        showUnavailable={false}
        hideInlineMobileUi
      />,
    ),
  );
  expect(container.querySelector("button")).toBeNull();
});

it.each([
  [390, 844, 1080, 1920],
  [844, 390, 1080, 1920],
  [390, 844, 1920, 1080],
  [844, 390, 1920, 1080],
  [390, 844, 1080, 1080],
  [390, 844, 720, 1920],
  [390, 844, 0, 0],
])(
  "fits %sx%s fullscreen around a %sx%s stream",
  async (width, height, videoWidth, videoHeight) => {
    fixture.window = { width, height };
    fixture.video = { width: videoWidth, height: videoHeight };
    fixture.fullscreen = true;
    if (width > height)
      fixture.insets = { top: 0, bottom: 21, left: 47, right: 0 };
    await act(async () => root.render(<Fullscreen src="test-stream" />));
    const video = container.querySelector("span")!.parentElement!;
    const style = JSON.parse(video.dataset.style!);
    const ratio =
      videoWidth > 0 && videoHeight > 0 ? videoWidth / videoHeight : 16 / 9;
    expect(style.width / style.height).toBeCloseTo(ratio);
    expect(style.left).toBeGreaterThanOrEqual(fixture.insets.left);
    expect(style.top).toBeGreaterThanOrEqual(fixture.insets.top);
    expect(style.left + style.width).toBeLessThanOrEqual(
      width - fixture.insets.right,
    );
    expect(style.top + style.height).toBeLessThanOrEqual(
      height - fixture.insets.bottom,
    );
    expect(
      Math.max(
        style.width / (width - fixture.insets.left - fixture.insets.right),
        style.height / (height - fixture.insets.top - fixture.insets.bottom),
      ),
    ).toBeCloseTo(1);
  },
);

it("keeps fullscreen fitted when the phone rotates during playback", async () => {
  fixture.fullscreen = true;
  await act(async () => root.render(<Fullscreen src="test-stream" />));
  fixture.window = { width: 844, height: 390 };
  fixture.insets = { top: 0, bottom: 21, left: 47, right: 0 };
  await act(async () => root.render(<Fullscreen src="test-stream" />));
  const video = container.querySelector("span")!.parentElement!;
  const style = JSON.parse(video.dataset.style!);
  const availableWidth = fixture.window.width - fixture.insets.left;
  const availableHeight = fixture.window.height - fixture.insets.bottom;
  expect(style.height).toBe(availableHeight);
  expect(style.width / style.height).toBeCloseTo(1080 / 1920);
  expect(style.left).toBeCloseTo(
    fixture.insets.left + (availableWidth - style.width) / 2,
  );
  expect(style.top).toBe(0);
});
