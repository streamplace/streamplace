import { makeLivestreamStore, type LivestreamStore } from "@streamplace/core";
import { act, type ComponentType, type ReactNode } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { Route } from "../../routes/$user/index";

const fixture = vi.hoisted(() => ({
  store: null as unknown as LivestreamStore,
  liveness: "live",
  navigate: vi.fn(),
  profiles: {},
}));

vi.mock("@tanstack/react-router", () => ({
  createFileRoute: () => (options: object) => ({
    options,
    useParams: () => ({ user: "streamer.example" }),
  }),
  useNavigate: () => fixture.navigate,
  Link: ({ children }: { children: ReactNode }) => <a>{children}</a>,
}));
vi.mock("@/hooks/actor-lookup", () => ({
  useActorLookup: () => ({ status: "found" }),
}));
vi.mock("@/hooks/use-livestream-store", () => ({
  useLivestreamStore: () => ({ store: fixture.store, ready: true }),
}));
vi.mock("@/hooks/use-liveness-state", () => ({
  useLivenessState: () => fixture.liveness,
}));
vi.mock("@/hooks/use-stream-avatar", () => ({
  useStreamAvatar: () => undefined,
}));
vi.mock("@/hooks/use-avatars", () => ({ default: () => fixture.profiles }));
vi.mock("@/contexts/fullscreen-context", () => ({
  useFullscreen: () => ({ theatre: false }),
}));
vi.mock("@/lib/store/hooks", () => ({
  useStreamplaceUrl: () => "https://node.example",
}));
vi.mock("@/components/stream/video-section", () => ({
  VideoSection: () => null,
}));
vi.mock("@/components/stream/chat-sidebar", () => ({
  ChatSidebar: () => null,
}));
vi.mock("@/components/stream/stream-info", () => ({
  StreamInfo: () => null,
  CopyButton: () => null,
  activityLabel: () => null,
}));
vi.mock("react-i18next", () => ({
  useTranslation: () => ({ t: (key: string) => key }),
}));

describe("stream chat during a teleport", () => {
  let container: HTMLDivElement;
  let root: Root;
  const StreamPage = Route.options.component as ComponentType;

  function startTeleport() {
    fixture.store.setState({
      activeTeleport: {
        $type: "place.stream.live.teleport",
        streamer: "did:plc:target",
        startsAt: new Date(
          Date.now() + 60_000,
        ).toISOString() as unknown as NonNullable<
          ReturnType<typeof fixture.store.getState>["activeTeleport"]
        >["startsAt"],
      },
    });
  }

  beforeEach(() => {
    vi.stubGlobal("IS_REACT_ACT_ENVIRONMENT", true);
    vi.stubGlobal("matchMedia", () => ({ matches: true }));
    localStorage.setItem("streamplace:chat-open", "false");
    fixture.store = makeLivestreamStore();
    fixture.liveness = "live";
    container = document.createElement("div");
    document.body.appendChild(container);
    root = createRoot(container);
  });

  afterEach(async () => {
    await act(async () => root.unmount());
    container.remove();
    localStorage.clear();
    vi.unstubAllGlobals();
  });

  it("reveals a teleport in closed chat without overwriting the saved preference", async () => {
    await act(async () => root.render(<StreamPage />));
    expect(container.querySelector('[aria-label="chat-open"]')).not.toBeNull();

    await act(async () => startTeleport());

    expect(container.querySelector('[aria-label="chat-close"]')).not.toBeNull();
    expect(localStorage.getItem("streamplace:chat-open")).toBe("false");
  });

  it("keeps the countdown accessible when liveness changes, and allows intentional closing", async () => {
    await act(async () => root.render(<StreamPage />));
    await act(async () => startTeleport());
    fixture.liveness = "offline";
    await act(async () => root.render(<StreamPage />));
    const close = container.querySelector<HTMLButtonElement>(
      '[aria-label="chat-close"]',
    );
    expect(close).not.toBeNull();

    await act(async () => close?.click());

    expect(container.querySelector('[aria-label="chat-open"]')).not.toBeNull();
  });
});
