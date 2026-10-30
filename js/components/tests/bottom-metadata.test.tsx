import { act, createElement } from "react";
import { createRoot } from "react-dom/client";
import { afterEach, beforeEach, expect, it, vi } from "vitest";
import { useStore } from "zustand";
import { BottomMetadata } from "../../app/components/mobile/bottom-metadata";
import { makeLivestreamStore } from "../../core/src/livestream-store/store";
import { statusColors } from "../src/lib/theme/tokens";

const fixture = vi.hoisted(() => ({
  store: null as ReturnType<typeof makeLivestreamStore> | null,
  mode: "live",
  did: "did:plc:viewer" as string | null,
  openURL: vi.fn(),
}));
// The store subscriptions, avatar ring, presence expiry and Details state run
// unchanged; native presentation bindings and unrelated actions are replaced.
function View({
  children,
  style,
  accessibilityLabel,
  accessibilityElementsHidden,
  testID,
}: any) {
  return createElement(
    "div",
    {
      "aria-label": accessibilityLabel,
      "data-testid": testID,
      hidden: accessibilityElementsHidden,
      "data-style": JSON.stringify(
        Object.assign({}, ...[style].flat(Infinity)),
      ),
    },
    children,
  );
}
function Pressable({
  children,
  onPress,
  accessibilityLabel,
  accessibilityState,
}: any) {
  return createElement(
    "button",
    {
      onClick: onPress,
      "aria-label": accessibilityLabel,
      "aria-expanded": accessibilityState?.expanded,
    },
    children,
  );
}
vi.mock("react-native", () => ({
  View,
  Text: View,
  ScrollView: ({ children }: any) =>
    createElement("div", { "data-testid": "scrolling-tags" }, children),
  Pressable,
  Platform: { OS: "web", select: () => null },
  Linking: { openURL: fixture.openURL },
}));
vi.mock("expo-image", () => ({ Image: () => null }));
vi.mock("react-native-reanimated", async () => {
  const { useRef } = await import("react");
  return {
    default: { View },
    useSharedValue: (value: unknown) => useRef({ value }).current,
    useAnimatedStyle: (style: () => unknown) => style(),
    withTiming: (value: unknown) => value,
  };
});
vi.mock("lucide-react-native", () => ({
  ChevronDown: () => null,
  ChevronUp: () => null,
  ChevronLeft: () => null,
  ChevronRight: () => null,
}));
vi.mock("@streamplace/core", () => import("../../core/src/lib/format-handle"));
vi.mock("../src/lib/theme/theme", async () => {
  const tokens = await import("../src/lib/theme/tokens");
  return {
    useTheme: () => ({
      theme: {
        ...tokens,
        colors: {
          live: tokens.statusColors.live,
          text2: tokens.textAlphas.dark[2],
          surface2: tokens.surfaces.dark[2],
          surface3: tokens.surfaces.dark[3],
          borderSubtle: tokens.borderAlphas.dark.subtle,
        },
      },
      zero: {
        border: { border: { borderColor: tokens.borderAlphas.dark.default } },
      },
    }),
  };
});
vi.mock("@streamplace/components", async () => {
  const atoms = await import("../src/lib/theme/atoms");
  return {
    zero: atoms,
    layout: atoms.layout,
    Avatar: (await import("../src/components/ui/avatar")).Avatar,
    Text: View,
    IconButton: Pressable,
    LiveBadge: () => <span>LIVE badge</span>,
    PlayerUI: { Viewers: () => <span>Fallback viewer count</span> },
    ContentWarnings: () => <span>Repeated content warnings</span>,
    ContentRights: ({ contentRights }: any) => (
      <span>{contentRights.license}</span>
    ),
    ShareSheet: () => <button>Share stream</button>,
    formatActivity: (activity: any) => activity.name,
    useTheme: (await import("../src/lib/theme/theme")).useTheme,
    useTranslation: () => ({
      t: (key: string, options?: { count: number }) =>
        key === "watching-count" || key === "here-count"
          ? `${options!.count} ${key === "watching-count" ? "watching" : "here"}`
          : key === "stream-details"
            ? "Stream details"
            : key === "less-details"
              ? "Less details"
              : "More details",
    }),
    useAuthor: () => useStore(fixture.store!, (state) => state.profile),
    useAvatar: () => null,
    useDID: () => fixture.did,
    useTitle: () =>
      useStore(fixture.store!, (state) => state.livestream?.record.title),
    useViews: () => useStore(fixture.store!, (state) => state.viewers),
    useLivestreamStore: (select: any) => useStore(fixture.store!, select),
    usePlayerStore: (select: any) => select({ mode: fixture.mode }),
  };
});
vi.mock("components/aqlink", () => ({ default: View }));
vi.mock("components/follow-button", () => ({
  default: () => <button>Following</button>,
}));
vi.mock("../../app/components/mobile/desktop-ui/kebab", () => ({
  KebabMenu: () => <button>More stream actions</button>,
}));
vi.mock("../../app/components/live-dashboard/livestream-panel", () => ({
  LANG_TAG_PREFIX: "lang:",
  LANGUAGES: [{ code: "en", native: "English" }],
}));

const tags = [
  "lang:en",
  "bog",
  "knight",
  "lgbtq",
  "transrights",
  "firstplay",
  "NOspoilers",
  "MAYBEbackseating",
  "HORROR",
];
let container: HTMLDivElement;
let root: ReturnType<typeof createRoot>;
beforeEach(() => {
  vi.stubGlobal("IS_REACT_ACT_ENVIRONMENT", true);
  vi.useFakeTimers();
  vi.setSystemTime(new Date("2026-10-30T18:00:00Z"));
  fixture.mode = "live";
  fixture.did = "did:plc:viewer";
  fixture.openURL.mockClear();
  fixture.store = makeLivestreamStore();
  fixture.store.setState({
    profile: {
      did: "did:plc:creator",
      handle: "creator.example",
      displayName: "Creator name",
    },
    viewers: 24,
    livestream: {
      uri: "at://did:plc:creator/place.stream.livestream/test",
      author: { did: "did:plc:creator" },
      record: { title: "A creator's live stream", tags },
    } as any,
    segment: {
      startTime: new Date(Date.now() - 2_000).toISOString(),
      duration: 1_000_000_000,
      contentWarnings: { warnings: ["language"] },
      contentRights: { license: "CC BY-NC-SA 4.0" },
    } as any,
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
async function render(compact = false) {
  await act(async () =>
    root.render(
      <BottomMetadata compact={compact} setShowChat={() => {}} showChat />,
    ),
  );
}
function hasRing() {
  const avatar = container.querySelector<HTMLElement>(
    '[aria-label="creator.example"]',
  )!;
  return JSON.parse(avatar.dataset.style!).borderColor === statusColors.live;
}
function expectNotLive() {
  expect(hasRing()).toBe(false);
  expect(container.textContent).not.toContain("watching");
  expect(container.textContent).not.toContain("Fallback viewer count");
}

it("shows the title, creator, all wrapping tags and quiet live count", async () => {
  await render();
  expect(container.textContent).toContain("A creator's live stream");
  expect(container.textContent).toContain("Creator name");
  for (const tag of tags)
    expect(container.textContent).toContain(
      tag === "lang:en" ? "English" : tag,
    );
  expect(container.querySelector('[data-testid="scrolling-tags"]')).toBeNull();
  expect(container.textContent).toContain("24 watching");
  expect(container.textContent).not.toContain("LIVE badge");
  expect(container.textContent).not.toContain("Repeated content warnings");
  expect(hasRing()).toBe(true);
});

it.each(["unknown", "invalid", "stale", "ended", "recording"])(
  "does not mark a %s stream as live",
  async (state) => {
    if (state === "unknown") fixture.store!.setState({ segment: null });
    if (state === "invalid")
      fixture.store!.setState({ segment: { startTime: "invalid" } as any });
    if (state === "stale")
      fixture.store!.setState({
        segment: {
          startTime: new Date(Date.now() - 60_000).toISOString(),
        } as any,
      });
    if (state === "ended")
      fixture.store!.setState({
        livestream: {
          ...fixture.store!.getState().livestream!,
          record: {
            ...fixture.store!.getState().livestream!.record,
            endedAt: new Date().toISOString(),
          },
        },
      });
    if (state === "recording") fixture.mode = "vod";
    await render();
    expectNotLive();
  },
);

it("expires live status without a new store event, then recovers on fresh activity", async () => {
  await render();
  await act(async () => vi.advanceTimersByTime(9_000));
  expectNotLive();
  await act(async () =>
    fixture.store!.setState({
      segment: {
        startTime: new Date().toISOString(),
        duration: 1_000_000_000,
      } as any,
    }),
  );
  expect(hasRing()).toBe(true);
  expect(container.textContent).toContain("24 watching");
});

it("accepts a fresh stream heartbeat when the last media segment is stale", async () => {
  const livestream = fixture.store!.getState().livestream!;
  fixture.store!.setState({
    segment: { startTime: new Date(Date.now() - 60_000).toISOString() } as any,
    livestream: {
      ...livestream,
      record: { ...livestream.record, lastSeenAt: new Date().toISOString() },
    },
  });
  await render();
  expect(hasRing()).toBe(true);
  await act(async () => vi.advanceTimersByTime(10_000));
  expectNotLive();
});

it("keeps the live signal while a newer segment replaces the expiry deadline", async () => {
  await render();
  await act(async () => vi.advanceTimersByTime(8_000));
  await act(async () =>
    fixture.store!.setState({
      segment: { startTime: new Date().toISOString() } as any,
    }),
  );
  await act(async () => vi.advanceTimersByTime(2_000));
  expect(hasRing()).toBe(true);
  await act(async () => vi.advanceTimersByTime(8_000));
  expectNotLive();
});

it("hides live status as soon as an active stream explicitly ends", async () => {
  await render();
  const livestream = fixture.store!.getState().livestream!;
  await act(async () =>
    fixture.store!.setState({
      livestream: {
        ...livestream,
        record: { ...livestream.record, endedAt: new Date().toISOString() },
      },
    }),
  );
  expectNotLive();
});

it.each([0, null])(
  "distinguishes a viewer count of %s from live status",
  async (count) => {
    fixture.store!.setState({ viewers: count });
    await render();
    expect(hasRing()).toBe(true);
    expect(container.textContent?.includes("watching")).toBe(count !== null);
    if (count === 0) expect(container.textContent).toContain("0 watching");
  },
);

it("keeps rights available under Details without repeating them by default", async () => {
  await render();
  expect(container.textContent).not.toContain("CC BY-NC-SA");
  const details = [...container.querySelectorAll("button")].find((button) =>
    /details/i.test(button.textContent ?? ""),
  );
  expect(details).toBeDefined();
  await act(async () => details!.click());
  expect(container.textContent).toContain("CC BY-NC-SA 4.0");
  await act(async () => details!.click());
  expect(container.textContent).not.toContain("CC BY-NC-SA");
});

it("does not offer empty Details when there are no displayable rights", async () => {
  fixture.store!.setState({
    segment: {
      startTime: new Date().toISOString(),
      contentRights: { creator: "did:plc:creator" },
    } as any,
  });
  await render();
  expect(container.textContent).not.toMatch(/details/i);
});

it("opens a compact summary into creator, full title and all tags, then closes it", async () => {
  await render(true);
  const summary = container.querySelector<HTMLButtonElement>(
    '[aria-label="Stream info"]',
  )!;
  expect(summary.textContent).toContain("A creator's live stream");
  expect(summary.textContent).toContain("@creator.example");
  const expand = container.querySelector<HTMLButtonElement>(
    '[aria-label="Expand stream info"]',
  )!;
  expect(expand.getAttribute("aria-expanded")).toBe("false");
  await act(async () => expand.click());
  const profile = container.querySelector<HTMLButtonElement>(
    '[aria-label="Open profile for @creator.example"]',
  )!;
  expect(profile.textContent).toContain("Creator name");
  expect(container.textContent).toContain("24 watching");
  expect(container.querySelector('[data-testid="scrolling-tags"]')).toBeNull();
  expect(container.textContent).not.toContain("Repeated content warnings");
  for (const tag of tags)
    expect(container.textContent).toContain(
      tag === "lang:en" ? "English" : tag,
    );
  expect(container.textContent).not.toContain("CC BY-NC-SA");
  const details = [...container.querySelectorAll("button")].find(
    (button) => button.textContent === "Stream details",
  )!;
  await act(async () => details.click());
  expect(container.textContent).toContain("CC BY-NC-SA 4.0");
  const collapse = container.querySelector<HTMLButtonElement>(
    '[aria-label="Collapse stream info"]',
  )!;
  await act(async () => collapse.click());
  expect(container.querySelector('[aria-label="Stream info"]')).not.toBeNull();
  expect(
    container
      .querySelector('[aria-label="Expand stream info"]')
      ?.getAttribute("aria-expanded"),
  ).toBe("false");
});

it.each([0, 3, null])(
  "uses offline room presence (%s), rather than stale playback viewers",
  async (presence) => {
    fixture.store!.setState({
      presence,
      websocketConnected: true,
      segment: {
        startTime: new Date(Date.now() - 60_000).toISOString(),
      } as any,
    });
    await render(true);
    expectNotLive();
    const expand = container.querySelector<HTMLButtonElement>(
      '[aria-label="Expand stream info"]',
    )!;
    await act(async () => expand.click());
    expect(container.textContent?.includes("here")).toBe(presence !== null);
    if (presence !== null)
      expect(container.textContent).toContain(`${presence} here`);
    expect(container.textContent).not.toContain("24 watching");
  },
);

it("does not show disconnected presence in the compact panel", async () => {
  fixture.store!.setState({
    presence: 3,
    websocketConnected: false,
    segment: null,
  });
  await render(true);
  await act(async () =>
    container
      .querySelector<HTMLButtonElement>('[aria-label="Expand stream info"]')!
      .click(),
  );
  expect(container.textContent).not.toContain("here");
});
