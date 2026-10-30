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
  mode: "live",
  status: "playing",
  duration: 36,
  playTime: 0,
  rendition: "audio",
  screenReader: false,
  canRotate: false,
  profile: { did: "did:plc:streamer", handle: "streamer.example" } as {
    did: string;
    handle: string;
  } | null,
  warnings: ["nudity"],
  openURL: vi.fn(),
  tap: (_event: unknown, _success: boolean) => {},
}));

// Only presentation bindings and external screen/stream inputs are replaced.
// Control selection, fullscreen sizing, and notification dismissal run unchanged.
function View({
  children,
  style,
  edges,
  pointerEvents,
  onFocus,
  onBlur,
  onPointerDown,
  "aria-hidden": ariaHidden,
}: {
  children?: ReactNode;
  style?: any;
  edges?: string[];
  pointerEvents?: string;
  onFocus?: () => void;
  onBlur?: () => void;
  onPointerDown?: () => void;
  "aria-hidden"?: boolean;
}) {
  const flattenedStyle = Object.assign({}, ...[style].flat(Infinity));
  return createElement(
    "div",
    {
      "data-style": JSON.stringify(flattenedStyle),
      "data-edges": edges?.join(","),
      "data-pointer-events": pointerEvents ?? flattenedStyle.pointerEvents,
      "aria-hidden": ariaHidden,
      onFocus,
      onBlur,
      onPointerDown,
    },
    children,
  );
}
function canPress(element: HTMLElement) {
  for (
    let node: HTMLElement | null = element;
    node;
    node = node.parentElement
  ) {
    const style = JSON.parse(node.dataset.style ?? "{}");
    if (node.dataset.pointerEvents === "none" || style.opacity === 0)
      return false;
  }
  return true;
}
vi.mock("react-native", () => ({
  AccessibilityInfo: {
    isScreenReaderEnabled: async () =>
      fixture.platform === "web" || fixture.screenReader,
    addEventListener: () => ({ remove() {} }),
  },
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
vi.mock("react-native-reanimated", async () => {
  const { useRef } = await import("react");
  return {
    default: { View, createAnimatedComponent: () => View },
    useSharedValue: (value: unknown) => useRef({ value }).current,
    useAnimatedStyle: (style: () => unknown) => style(),
    withTiming: (
      value: unknown,
      _options: unknown,
      done?: (finished: boolean) => void,
    ) => {
      done?.(true);
      return value;
    },
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
  };
});
vi.mock("react-native-safe-area-context", () => ({
  useSafeAreaInsets: () => fixture.insets,
  SafeAreaView: View,
}));
vi.mock("react-native-gesture-handler", () => {
  const gesture = {
    runOnJS() {
      return this;
    },
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
      Tap: () => ({
        ...gesture,
        onEnd(handler: typeof fixture.tap) {
          fixture.tap = handler;
          return this;
        },
      }),
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
vi.mock("components/ui/gradient", () => ({ default: View }));
vi.mock("utils/emoji", () => ({ useEmojiData: () => ({}) }));
vi.mock(
  "@streamplace/components/src/ui",
  () => import("../src/lib/theme/atoms"),
);
vi.mock(
  "../../app/components/mobile/desktop-ui/index",
  () => import("../../app/components/mobile/desktop-ui/bottom-controls"),
);
vi.mock("../../app/components/mobile/desktop-ui/mu", () => ({ Mu: View }));
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
  Fullscreen: View,
  PictureInPicture2: View,
  Volume2: View,
  VolumeX: View,
}));
vi.mock("@react-navigation/native", () => ({ useNavigation: () => ({}) }));
vi.mock("hooks/useLiveUser", () => ({ useLiveUser: () => false }));
vi.mock("hooks/useSidebarControl", () => ({
  useSidebarControl: () => ({ contentMargin: 0, isActive: false }),
}));
vi.mock("store/hooks", () => ({
  useUserProfile: () => null,
  useIsSidebarCollapsed: () => false,
}));
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
vi.mock("../src/components/mobile-player/playback-session", () => ({
  usePlaybackSession: () => {},
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
      ContextMenu: ({ onOpenChange }: any) => (
        <>
          <button onClick={() => onOpenChange?.(true)}>Open settings</button>
          <button onClick={() => onOpenChange?.(false)}>Close settings</button>
        </>
      ),
      AutoplayButton: () => null,
      CountdownOverlay: () => null,
      LoadingOverlay: () => null,
      SeekBar: ({ onScrubbingChange }: any) => (
        <>
          <button onClick={() => onScrubbingChange(true)}>Begin seeking</button>
          <button onClick={() => onScrubbingChange(false)}>End seeking</button>
        </>
      ),
      VodControls: ({ children }: any) => (
        <>
          <button>Recorded playback controls</button>
          {children}
        </>
      ),
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
        mode: fixture.mode,
        status: fixture.status,
        duration: fixture.duration,
        playTime: fixture.playTime,
        selectedRendition: fixture.rendition,
      }),
    usePlayerDimensions: () => fixture.window,
    useSegmentDimensions: () => fixture.video,
    useLivestreamStore: (select?: (state: any) => unknown) =>
      select?.({
        segment: { contentWarnings: { warnings: fixture.warnings } },
      }),
    PlayerStatus: { PLAYING: "playing", PAUSE: "pause" },
    useLivestreamInfo: () => ({ ingest: null, profile: fixture.profile }),
    useAuthor: () => fixture.profile,
    useTitle: () => "From the studio",
    useAvatar: () => undefined,
    useCameraToggle: () => ({}),
    useLivestream: () => null,
    useMuted: () => false,
    useSetMuted: () => () => {},
    useVolume: () => 1,
    useSetVolume: () => () => {},
    Slider: { Root: View, Track: View, Range: View, Thumb: View },
    useRotation: () => ({
      canRotate: fixture.canRotate,
      currentOrientation: 1,
    }),
    useNetworkProfileUrl: () => () => "https://network.example/streamer",
    Text: ({ children }: { children: ReactNode }) => <span>{children}</span>,
    Avatar: () => <span>Avatar</span>,
    ShareSheet: () => <span>Share</span>,
    Toast: () => null,
    ContentWarningBadge: () => <span>Intended for certain audiences</span>,
    useDanmuEnabled: () => false,
    useDanmuUnlocked: () => false,
    useSetDanmuEnabled: () => () => {},
    useDanmuOpacity: () => 1,
    useDanmuSpeed: () => 1,
    useDanmuLaneCount: () => 1,
    useDanmuMaxMessages: () => 1,
    DanmuOverlay: () => null,
    Chat: () => null,
    Loader: () => null,
    IconButton: ({ children, accessibilityLabel }: any) => (
      <button aria-label={accessibilityLabel}>{children}</button>
    ),
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
  fixture.mode = "live";
  fixture.status = "playing";
  fixture.playTime = 0;
  fixture.rendition = "audio";
  fixture.screenReader = false;
  fixture.canRotate = false;
  fixture.profile = { did: "did:plc:streamer", handle: "streamer.example" };
  fixture.warnings = ["nudity"];
  fixture.openURL.mockClear();
  container = document.createElement("div");
  document.body.appendChild(container);
  root = createRoot(container);
});

it.each(["ios", "web"])(
  "puts the streamer profile above the audience notice on %s",
  async (platform) => {
    fixture.platform = platform;
    const { MobileUi } = await vi.importActual<
      typeof import("../../app/components/mobile/ui")
    >("../../app/components/mobile/ui");
    await act(async () => root.render(<MobileUi />));
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
  container.remove();
  if (streamNotificationManager.getAll().length)
    streamNotificationManager.hide("pinned-comment");
  vi.restoreAllMocks();
  vi.useRealTimers();
  vi.unstubAllGlobals();
});

it("allows six seconds to use playing controls before they fade", async () => {
  vi.useFakeTimers();
  fixture.mode = "vod";
  fixture.rendition = "source";
  const { MobileUi } = await vi.importActual<
    typeof import("../../app/components/mobile/ui")
  >("../../app/components/mobile/ui");
  await act(async () => root.render(<MobileUi hideMobileChat />));
  const controls = [...container.querySelectorAll("button")].find(
    (button) => button.textContent === "Recorded playback controls",
  )!;
  await act(async () => vi.advanceTimersByTime(5900));
  expect(canPress(controls)).toBe(true);
  await act(async () => vi.advanceTimersByTime(200));
  expect(canPress(controls)).toBe(false);
});

it.each(["paused", "ended"])(
  "keeps %s recorded controls visible after the inactivity timeout",
  async (state) => {
    vi.useFakeTimers();
    fixture.mode = "vod";
    fixture.status = state === "paused" ? "pause" : "suspend";
    if (state === "ended") fixture.playTime = fixture.duration;
    fixture.rendition = "source";
    const { MobileUi } = await vi.importActual<
      typeof import("../../app/components/mobile/ui")
    >("../../app/components/mobile/ui");
    await act(async () => root.render(<MobileUi hideMobileChat />));
    await act(async () => vi.advanceTimersByTime(7000));
    const controls = [...container.querySelectorAll("button")].find(
      (button) => button.textContent === "Recorded playback controls",
    )!;
    expect(canPress(controls)).toBe(true);
  },
);

it("keeps playback controls discoverable with a screen reader enabled", async () => {
  vi.useFakeTimers();
  fixture.mode = "vod";
  fixture.rendition = "source";
  fixture.screenReader = true;
  const { MobileUi } = await vi.importActual<
    typeof import("../../app/components/mobile/ui")
  >("../../app/components/mobile/ui");
  await act(async () => root.render(<MobileUi hideMobileChat />));
  await act(async () => vi.advanceTimersByTime(7000));
  const controls = [...container.querySelectorAll("button")].find(
    (button) => button.textContent === "Recorded playback controls",
  )!;
  expect(canPress(controls)).toBe(true);
});

it.each(["live", "vod"])(
  "fades web %s controls despite the screen-reader API's constant true result",
  async (mode) => {
    vi.useFakeTimers();
    fixture.platform = "web";
    fixture.mode = mode;
    fixture.rendition = "source";
    const { MobileUi } = await vi.importActual<
      typeof import("../../app/components/mobile/ui")
    >("../../app/components/mobile/ui");
    await act(async () => root.render(<MobileUi hideMobileChat />));
    const back = container.querySelector<HTMLElement>('[aria-label="Back"]')!;
    expect(canPress(back)).toBe(true);
    await act(async () => vi.advanceTimersByTime(6100));
    expect(canPress(back)).toBe(false);
  },
);

it("names the native fullscreen action", async () => {
  fixture.canRotate = true;
  const { MobileUi } = await vi.importActual<
    typeof import("../../app/components/mobile/ui")
  >("../../app/components/mobile/ui");
  await act(async () => root.render(<MobileUi hideMobileChat />));
  expect(
    container.querySelector('[aria-label="Enter fullscreen"]'),
  ).not.toBeNull();
});

it("requires revealing faded recorded controls before they accept a press", async () => {
  vi.useFakeTimers();
  fixture.mode = "vod";
  fixture.rendition = "source";
  const { MobileUi } = await vi.importActual<
    typeof import("../../app/components/mobile/ui")
  >("../../app/components/mobile/ui");
  await act(async () => root.render(<MobileUi hideMobileChat />));
  await act(async () => vi.advanceTimersByTime(7000));
  const controls = [...container.querySelectorAll("button")].find(
    (button) => button.textContent === "Recorded playback controls",
  )!;
  expect(canPress(controls)).toBe(false);
});

it.each(["vod", "live"])(
  "fades and reveals back with the %s controls",
  async (mode) => {
    vi.useFakeTimers();
    fixture.mode = mode;
    fixture.rendition = "source";
    const { MobileUi } = await vi.importActual<
      typeof import("../../app/components/mobile/ui")
    >("../../app/components/mobile/ui");
    await act(async () => root.render(<MobileUi hideMobileChat />));
    const back = container.querySelector<HTMLElement>('[aria-label="Back"]');
    expect(back).not.toBeNull();
    expect(canPress(back!)).toBe(true);
    await act(async () => vi.advanceTimersByTime(7000));
    expect(canPress(back!)).toBe(false);
    await act(async () => fixture.tap({}, true));
    expect(canPress(back!)).toBe(true);
  },
);

it.each(["playing", "pause"])(
  "toggles %s controls on a completed tap and starts a fresh timeout on reveal",
  async (status) => {
    vi.useFakeTimers();
    fixture.mode = "vod";
    fixture.status = status;
    fixture.rendition = "source";
    const { MobileUi } = await vi.importActual<
      typeof import("../../app/components/mobile/ui")
    >("../../app/components/mobile/ui");
    await act(async () => root.render(<MobileUi hideMobileChat />));
    const back = container.querySelector<HTMLElement>('[aria-label="Back"]')!;
    const controls = [...container.querySelectorAll("button")].find(
      (button) => button.textContent === "Recorded playback controls",
    )!;
    await act(async () => fixture.tap({}, false));
    expect(canPress(back)).toBe(true);
    expect(canPress(controls)).toBe(true);
    await act(async () => vi.advanceTimersByTime(3000));
    await act(async () => fixture.tap({}, true));
    expect(canPress(back)).toBe(false);
    expect(canPress(controls)).toBe(false);
    await act(async () => vi.advanceTimersByTime(500));
    await act(async () => fixture.tap({}, true));
    await act(async () => vi.advanceTimersByTime(5900));
    expect(canPress(back)).toBe(true);
    expect(canPress(controls)).toBe(true);
    await act(async () => vi.advanceTimersByTime(200));
    expect(canPress(back)).toBe(status === "pause");
    expect(canPress(controls)).toBe(status === "pause");
  },
);

it.each([
  ["Open settings", "Close settings"],
  ["Begin seeking", "End seeking"],
])(
  "holds playback controls during %s and resumes the timeout afterwards",
  async (begin, end) => {
    vi.useFakeTimers();
    fixture.mode = "vod";
    fixture.rendition = "source";
    const { MobileUi } = await vi.importActual<
      typeof import("../../app/components/mobile/ui")
    >("../../app/components/mobile/ui");
    await act(async () => root.render(<MobileUi hideMobileChat />));
    const button = (label: string) =>
      [...container.querySelectorAll("button")].find(
        (node) => node.textContent === label,
      )!;
    await act(async () => button(begin).click());
    await act(async () => vi.advanceTimersByTime(7000));
    expect(canPress(button("Recorded playback controls"))).toBe(true);
    await act(async () => button(end).click());
    await act(async () => vi.advanceTimersByTime(7000));
    expect(canPress(button("Recorded playback controls"))).toBe(false);
  },
);

it("leaves recorded creator identity and sharing to the watch-page metadata", async () => {
  fixture.mode = "vod";
  const { MobileUi } = await vi.importActual<
    typeof import("../../app/components/mobile/ui")
  >("../../app/components/mobile/ui");
  await act(async () => root.render(<MobileUi hideMobileChat />));
  expect(
    container.querySelector('[aria-label="Open profile for streamer.example"]'),
  ).toBeNull();
  expect(container.textContent).not.toContain("Share");
});

it.each(["live", "vod"])(
  "keeps mute and fullscreen available on narrow web %s playback",
  async (mode) => {
    fixture.platform = "web";
    fixture.mode = mode;
    const { MobileUi } = await vi.importActual<
      typeof import("../../app/components/mobile/ui")
    >("../../app/components/mobile/ui");
    await act(async () => root.render(<MobileUi hideMobileChat />));
    for (const label of ["Back", "Mute", "Enter fullscreen"]) {
      const control = container.querySelector<HTMLElement>(
        `[aria-label="${label}"]`,
      );
      expect(control, label).not.toBeNull();
      expect(canPress(control!)).toBe(true);
      await act(async () => fixture.tap({}, true));
      expect(canPress(control!)).toBe(false);
      expect(control!.closest('[aria-hidden="true"]')).not.toBeNull();
      await act(async () => fixture.tap({}, true));
      expect(canPress(control!)).toBe(true);
    }
  },
);

it.each(["live", "vod"])(
  "allows web %s controls to hide after pointer focus, then holds keyboard interaction",
  async (mode) => {
    vi.useFakeTimers();
    fixture.platform = "web";
    fixture.mode = mode;
    fixture.rendition = "source";
    const { MobileUi } = await vi.importActual<
      typeof import("../../app/components/mobile/ui")
    >("../../app/components/mobile/ui");
    await act(async () => root.render(<MobileUi hideMobileChat />));
    const mute = container.querySelector<HTMLButtonElement>(
      '[aria-label="Mute"]',
    )!;
    await act(async () => {
      mute.dispatchEvent(new Event("pointerdown", { bubbles: true }));
      mute.focus();
    });
    await act(async () => vi.advanceTimersByTime(6100));
    expect(canPress(mute)).toBe(false);
    await act(async () =>
      document.dispatchEvent(new KeyboardEvent("keydown", { key: "Enter" })),
    );
    await act(async () => vi.advanceTimersByTime(7000));
    expect(canPress(mute)).toBe(true);
    await act(async () => mute.blur());
    await act(async () => vi.advanceTimersByTime(6100));
    expect(canPress(mute)).toBe(false);
  },
);

it("shows the title and handle in landscape without a recorded-chat toggle", async () => {
  fixture.window = { width: 844, height: 390 };
  fixture.mode = "vod";
  const { MobileUi } = await vi.importActual<
    typeof import("../../app/components/mobile/ui")
  >("../../app/components/mobile/ui");
  await act(async () =>
    root.render(<MobileUi hideMobileChat showChat setShowChat={() => {}} />),
  );
  expect(container.textContent).toContain("From the studio");
  expect(container.textContent).toContain("@streamer.example");
  expect(container.querySelector('[aria-label="Hide chat"]')).toBeNull();
  expect(container.querySelector('[aria-label="Show chat"]')).toBeNull();
});

it("keeps wide web identity below the player until fullscreen", async () => {
  fixture.platform = "web";
  fixture.window = { width: 1024, height: 768 };
  fixture.mode = "vod";
  const { MobileUi } = await vi.importActual<
    typeof import("../../app/components/mobile/ui")
  >("../../app/components/mobile/ui");
  await act(async () => root.render(<MobileUi hideMobileChat />));
  expect(container.textContent).not.toContain("From the studio");
  fixture.fullscreen = true;
  await act(async () => root.render(<MobileUi hideMobileChat />));
  expect(container.textContent).toContain("From the studio");
  expect(container.textContent).toContain("@streamer.example");
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

it.each([
  [1440, 1100, 1920, 1080],
  [1920, 800, 2560, 1080],
  [1440, 1100, 1080, 1920],
])(
  "fits a %sx%s desktop around a %sx%s stream without an outer gutter",
  async (windowWidth, windowHeight, videoWidth, videoHeight) => {
    fixture.platform = "web";
    fixture.window = { width: windowWidth, height: windowHeight };
    fixture.video = { width: videoWidth, height: videoHeight };
    fixture.insets = { top: 0, bottom: 0, left: 0, right: 0 };
    await act(async () =>
      root.render(
        <PlayerInner
          showChat={false}
          setShowChat={() => {}}
          showUnavailable={false}
        />,
      ),
    );
    const controls = container.querySelector("button")!;
    let frame = controls.parentElement!;
    while (typeof JSON.parse(frame.dataset.style!).width !== "number")
      frame = frame.parentElement!;
    const style = JSON.parse(frame.dataset.style!);
    const scrollStyle = JSON.parse(frame.parentElement!.dataset.style!);
    expect(scrollStyle.maxWidth).toBe(windowWidth);
    expect(style.width).toBeLessThanOrEqual(windowWidth);
    expect(style.height).toBeLessThanOrEqual(windowHeight);
    if (videoWidth / videoHeight > windowWidth / windowHeight)
      expect(style.width).toBe(windowWidth);
    expect(style.width / style.height).toBeCloseTo(videoWidth / videoHeight);
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
