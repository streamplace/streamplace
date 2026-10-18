import { makeLivestreamStore } from "@streamplace/core";
import { act } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { StreamNotificationProvider } from "./stream-notification-provider";
import { StreamNotifications } from "./stream-notifications";

const { mockUseSession } = vi.hoisted(() => ({ mockUseSession: vi.fn() }));
vi.mock("../../hooks/use-avatars", () => ({ default: () => ({}) }));
vi.mock("../../lib/session", () => ({
  useSession: mockUseSession,
}));
vi.mock("react-i18next", () => ({
  useTranslation: () => ({ t: (key: string) => key }),
}));

describe("TeleportNotification", () => {
  let container: HTMLDivElement;
  let root: Root;
  let store: ReturnType<typeof makeLivestreamStore>;

  function render() {
    root.render(<StreamNotifications store={store} />);
  }

  function teleport(startsAt: string) {
    return {
      $type: "place.stream.live.teleport" as const,
      streamer: "did:plc:target" as const,
      startsAt: startsAt as unknown as NonNullable<
        ReturnType<typeof store.getState>["activeTeleport"]
      >["startsAt"],
    };
  }

  beforeEach(() => {
    vi.stubGlobal("IS_REACT_ACT_ENVIRONMENT", true);
    vi.useFakeTimers();
    mockUseSession.mockReturnValue({ did: null, pdsAgent: null });
    store = makeLivestreamStore();
    container = document.createElement("div");
    document.body.appendChild(container);
    root = createRoot(container);
  });

  afterEach(async () => {
    await act(async () => root.unmount());
    container.remove();
    vi.useRealTimers();
    vi.unstubAllGlobals();
  });

  it("keeps hook order when a teleport countdown expires or has an invalid time", async () => {
    store.setState({
      activeTeleport: teleport(new Date(Date.now() + 1_000).toISOString()),
    });
    await act(async () => render());

    await act(async () => {
      await vi.advanceTimersByTimeAsync(1_000);
    });
    expect(container.textContent).toBe("");

    await act(async () => {
      store.setState({
        activeTeleport: teleport("invalid"),
      });
    });

    expect(container.textContent).toBe("");
  });

  function pin(text: string, expiresAt?: string) {
    return {
      uri: `at://did:plc:streamer/place.stream.chat.pinnedRecord/${text}`,
      record: { expiresAt },
      message: {
        author: { did: "did:plc:author", handle: "author.example" },
        record: { text },
      },
    } as never;
  }

  it("keeps a pin visible when the teleport countdown expires", async () => {
    store.setState({
      pinnedComment: pin("pinned message"),
      activeTeleport: teleport(new Date(Date.now() + 1_000).toISOString()),
    });
    await act(async () => render());

    expect(container.textContent).toContain("pinned message");
    expect(container.textContent).toContain("teleporting-to");

    await act(async () => vi.advanceTimersByTimeAsync(1_000));

    expect(container.textContent).toContain("pinned message");
    expect(container.textContent).not.toContain("teleporting-to");
  });

  it("shares important notices with the video while keeping pins in chat", async () => {
    store.setState({
      pinnedComment: pin("pinned message"),
      activeTeleport: teleport(new Date(Date.now() + 1_000).toISOString()),
    });
    await act(async () =>
      root.render(
        <StreamNotifications store={store}>
          <section id="video">
            <StreamNotificationProvider importantOnly position="top-right" />
          </section>
          <section id="chat">
            <StreamNotificationProvider />
          </section>
        </StreamNotifications>,
      ),
    );

    expect(container.querySelector("#video")?.textContent).toContain(
      "teleporting-to",
    );
    expect(container.querySelector("#video")?.textContent).not.toContain(
      "pinned message",
    );
    expect(container.querySelector("#chat")?.textContent).toContain(
      "pinned message",
    );
    expect(container.querySelector("#chat")?.textContent).toContain(
      "teleporting-to",
    );

    await act(async () => vi.advanceTimersByTimeAsync(1_000));
    expect(container.querySelector("#video")?.textContent).toBe("");
    expect(container.querySelector("#chat")?.textContent).toContain(
      "pinned message",
    );
  });

  it("does not clear a newer teleport when an older cancellation completes", async () => {
    let finishDelete!: () => void;
    const deletion = new Promise<void>((resolve) => {
      finishDelete = resolve;
    });
    mockUseSession.mockReturnValue({
      did: "did:plc:source",
      pdsAgent: { client: { delete: () => deletion } },
    });
    store.setState({
      livestream: { author: { did: "did:plc:source" } } as never,
      activeTeleport: teleport(new Date(Date.now() + 10_000).toISOString()),
      activeTeleportUri: "at://did:plc:source/place.stream.live.teleport/old",
      activeTeleportCID: "old-cid",
    });
    await act(async () => render());
    await act(async () => container.querySelector("button")?.click());
    const newer = teleport(new Date(Date.now() + 20_000).toISOString());
    await act(async () =>
      store.setState({
        activeTeleport: newer,
        activeTeleportUri: "at://did:plc:source/place.stream.live.teleport/new",
        activeTeleportCID: "new-cid",
      }),
    );

    await act(async () => finishDelete());

    expect(store.getState().activeTeleport).toBe(newer);
    expect(store.getState().activeTeleportCID).toBe("new-cid");
  });

  it("does not send notification clicks to the player behind it", async () => {
    let playerClicks = 0;
    store.setState({
      activeTeleport: teleport(new Date(Date.now() + 10_000).toISOString()),
    });
    await act(async () =>
      root.render(
        <section onClick={() => playerClicks++}>
          <StreamNotifications store={store} />
        </section>,
      ),
    );
    const notice = container.querySelector("p");
    expect(notice).not.toBeNull();
    await act(async () => notice?.click());
    expect(playerClicks).toBe(0);
  });

  it("keeps teleports visible when pins are hidden", async () => {
    store.setState({
      pinnedComment: pin("hidden pin"),
      activeTeleport: teleport(new Date(Date.now() + 10_000).toISOString()),
    });

    await act(async () =>
      root.render(
        <StreamNotifications store={store} showPinnedComment={false} />,
      ),
    );

    expect(container.textContent).not.toContain("hidden pin");
    expect(container.textContent).toContain("teleporting-to");
  });

  it("isolates notifications and dismissal between chats", async () => {
    const otherStore = makeLivestreamStore();
    store.setState({ pinnedComment: pin("first pin") });
    otherStore.setState({ pinnedComment: pin("second pin") });
    await act(async () =>
      root.render(
        <>
          <section id="first">
            <StreamNotifications store={store} />
          </section>
          <section id="second">
            <StreamNotifications store={otherStore} />
          </section>
        </>,
      ),
    );

    expect(container.querySelector("#first")?.textContent).not.toContain(
      "second pin",
    );
    expect(container.querySelector("#second")?.textContent).not.toContain(
      "first pin",
    );
    await act(async () =>
      container
        .querySelector<HTMLButtonElement>(
          '#first [aria-label="chat-dismiss-pinned"]',
        )
        ?.click(),
    );
    expect(container.querySelector("#first")?.textContent).toBe("");
    expect(container.querySelector("#second")?.textContent).toContain(
      "second pin",
    );
  });
});
