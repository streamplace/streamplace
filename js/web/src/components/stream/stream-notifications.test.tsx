import { makeLivestreamStore } from "@streamplace/core";
import { act } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { StreamNotifications } from "./stream-notifications";

const { mockHide } = vi.hoisted(() => ({ mockHide: vi.fn() }));
vi.mock("../../hooks/use-avatars", () => ({ default: () => ({}) }));
vi.mock("../../lib/session", () => ({
  useSession: () => ({ did: null, pdsAgent: null }),
}));
vi.mock("../../lib/stream-notification", () => ({
  streamNotification: { hide: mockHide, show: vi.fn() },
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
    store = makeLivestreamStore();
    container = document.createElement("div");
    document.body.appendChild(container);
    root = createRoot(container);
  });

  afterEach(async () => {
    await act(async () => root.unmount());
    container.remove();
    mockHide.mockReset();
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
    expect(mockHide).toHaveBeenCalledWith("teleport");

    await act(async () => {
      store.setState({
        activeTeleport: teleport("invalid"),
      });
    });

    expect(container.textContent).toBe("");
  });
});
