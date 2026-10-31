import { act, createElement } from "react";
import { createRoot } from "react-dom/client";
import { afterEach, beforeEach, expect, it, vi } from "vitest";
import { VodMobileMetadata } from "../src/components/vod/vod-mobile-metadata";

const fixture = vi.hoisted(() => ({
  width: 390,
  hasAuthor: true,
  branding: undefined as Record<string, { data: string }> | undefined,
  openURL: vi.fn(),
  author: {
    did: "did:plc:creator",
    handle: "creator.example",
    displayName: "Creator display name",
  },
}));
vi.mock("@streamplace/core", () => import("../../core/src/lib/format-handle"));
function View({ children }: any) {
  return createElement("div", null, children);
}
vi.mock("react-native", () => ({
  View,
  Linking: { openURL: fixture.openURL },
  Pressable: ({ children, accessibilityLabel, onPress }: any) =>
    createElement(
      "button",
      { "aria-label": accessibilityLabel, onClick: onPress },
      children,
    ),
  Platform: { OS: "ios" },
  useWindowDimensions: () => ({ width: fixture.width, height: 844 }),
}));
vi.mock("lucide-react-native", () => ({ Share2: () => null }));
vi.mock("../src/hooks/useAuthor", () => ({
  useAuthor: () => (fixture.hasAuthor ? fixture.author : null),
}));
vi.mock("../src/hooks/useAvatar", () => ({ useAvatar: () => null }));
vi.mock("../src/hooks/useTitle", () => ({ useTitle: () => "Studio notes" }));
vi.mock("../src/streamplace-store", async () => ({
  useUrl: () => "https://stream.place",
  useNetworkProfileUrl: (await import("../src/streamplace-store/branding"))
    .useNetworkProfileUrl,
}));
vi.mock("../src/streamplace-store/streamplace-store", () => ({
  useStreamplaceStore: (select: any) => select({ branding: fixture.branding }),
}));
vi.mock("../src/streamplace-store/xrpc", () => ({}));
vi.mock("../src/storage", () => ({ default: {} }));
vi.mock("../src/video-store/video-store", () => ({
  useVideoStore: (select: any) =>
    select({
      aturi: "at://did:plc:creator/place.stream.video/recording-1",
      video: {
        uri: "at://did:plc:creator/place.stream.video/recording-1",
        author: fixture.author,
      },
    }),
}));
vi.mock("../src/ui", async () => {
  const tokens = await import("../src/lib/theme/tokens");
  return {
    ...(await import("../src/lib/theme/atoms")),
    useTheme: () => ({ theme: tokens }),
  };
});
vi.mock("../src/components/ui", () => ({ Avatar: () => null, Button: View }));
vi.mock("../src/components/ui/text", () => ({
  Text: ({ children }: any) => createElement("span", null, children),
}));
vi.mock("../src/components/vod/like-button", () => ({
  LikeButton: () => createElement("button", null, "Like video"),
}));
vi.mock("../src/components/share/sharesheet", () => ({
  ShareSheet: ({ target, children }: any) =>
    createElement("a", { href: target.url }, children ?? "Share"),
}));

let container: HTMLDivElement;
let root: ReturnType<typeof createRoot>;
beforeEach(() => {
  vi.stubGlobal("IS_REACT_ACT_ENVIRONMENT", true);
  container = document.createElement("div");
  root = createRoot(container);
  fixture.hasAuthor = true;
  fixture.branding = undefined;
  fixture.openURL.mockClear();
});
afterEach(async () => {
  await act(async () => root.unmount());
  vi.unstubAllGlobals();
});
it.each([390, 844])(
  "uses a single creator handle at %s pixels while retaining the video share target",
  async (width) => {
    fixture.width = width;
    await act(async () => root.render(<VodMobileMetadata />));
    expect(container.textContent).toContain("Studio notes");
    expect(container.textContent).toContain("@creator.example");
    expect(container.textContent).not.toContain("Creator display name");
    expect(container.querySelector("a")!.getAttribute("href")).toBe(
      "https://stream.place/creator.example/video/recording-1",
    );
    const profile = container.querySelector<HTMLButtonElement>(
      '[aria-label="Open profile for creator.example"]',
    );
    expect(profile).not.toBeNull();
    expect(profile!.textContent).toContain("@creator.example");
    await act(async () => profile!.click());
    expect(fixture.openURL).toHaveBeenCalledWith(
      "https://bsky.app/profile/creator.example",
    );
  },
);

it("opens the configured network profile using the recording's fallback author", async () => {
  fixture.hasAuthor = false;
  fixture.branding = {
    networkProfileUrl: { data: "https://profiles.example/{did}/{handle}" },
  };
  await act(async () => root.render(<VodMobileMetadata />));
  const profile = container.querySelector<HTMLButtonElement>(
    '[aria-label="Open profile for creator.example"]',
  );
  expect(profile).not.toBeNull();
  await act(async () => profile!.click());
  expect(fixture.openURL).toHaveBeenCalledWith(
    "https://profiles.example/did%3Aplc%3Acreator/creator.example",
  );
});
