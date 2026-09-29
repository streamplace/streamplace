import {
  handleWebSocketMessages,
  makeLivestreamStore,
  type LivestreamStore,
} from "@streamplace/core";
import { act } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { useCanModerate } from "./use-can-moderate";

const { mockUseSession } = vi.hoisted(() => ({ mockUseSession: vi.fn() }));

vi.mock("@/lib/session", () => ({ useSession: mockUseSession }));

const STREAMER = "did:plc:streamer";
const MODERATOR = "did:plc:moderator";

function PermissionProbe({ store }: { store: LivestreamStore }) {
  const { canBan } = useCanModerate(store);
  return <span>{canBan ? "allowed" : "denied"}</span>;
}

describe("useCanModerate", () => {
  let container: HTMLDivElement;
  let root: Root | undefined;

  beforeEach(() => {
    vi.useFakeTimers();
    vi.setSystemTime(new Date("2030-01-01T00:00:00.000Z"));
    vi.stubGlobal("IS_REACT_ACT_ENVIRONMENT", true);
    container = document.createElement("div");
    document.body.appendChild(container);
    root = undefined;
  });

  afterEach(async () => {
    if (root) {
      await act(async () => root?.unmount());
    }
    container.remove();
    mockUseSession.mockReset();
    vi.useRealTimers();
    vi.unstubAllGlobals();
  });

  it("revokes expired controls without waiting for another render", async () => {
    const expirationTime = new Date(Date.now() + 1_000).toISOString();
    const pdsAgent = {
      did: MODERATOR,
      com: {
        atproto: {
          repo: {
            listRecords: async () => ({
              data: {
                records: [
                  {
                    uri: `at://${STREAMER}/place.stream.moderation.permission/3kq`,
                    value: {
                      $type: "place.stream.moderation.permission",
                      moderator: MODERATOR,
                      permissions: ["ban"],
                      createdAt: "2029-01-01T00:00:00.000Z",
                      expirationTime,
                    },
                  },
                ],
              },
            }),
          },
        },
      },
    };
    mockUseSession.mockReturnValue({ pdsAgent, did: MODERATOR });

    const store = makeLivestreamStore();
    store.setState({ livestream: { author: { did: STREAMER } } as any });
    root = createRoot(container);

    await act(async () => {
      root?.render(<PermissionProbe store={store} />);
      await Promise.resolve();
      await Promise.resolve();
    });

    expect(container.textContent).toBe("allowed");

    await act(async () => {
      await vi.advanceTimersByTimeAsync(1_001);
    });

    expect(container.textContent).toBe("denied");
  });

  it("does not restore a deleted grant missing from the initial store", async () => {
    const uri = `at://${STREAMER}/place.stream.moderation.permission/3kq`;
    const permission = {
      $type: "place.stream.moderation.permission",
      moderator: MODERATOR,
      permissions: ["ban"],
      createdAt: "2029-01-01T00:00:00.000Z",
      uri,
    };
    let resolveListRecords!: (value: {
      data: { records: Array<{ uri: string; value: typeof permission }> };
    }) => void;
    const pdsAgent = {
      did: MODERATOR,
      com: {
        atproto: {
          repo: {
            listRecords: () =>
              new Promise((resolve) => {
                resolveListRecords = resolve;
              }),
          },
        },
      },
    };
    mockUseSession.mockReturnValue({ pdsAgent, did: MODERATOR });

    const store = makeLivestreamStore();
    store.setState({ livestream: { author: { did: STREAMER } } as any });
    root = createRoot(container);

    await act(async () => {
      root?.render(<PermissionProbe store={store} />);
    });
    expect(container.textContent).toBe("denied");

    await act(async () => {
      store.setState((state) =>
        handleWebSocketMessages(state, [
          {
            $type: "place.stream.moderation.permission",
            deleted: true,
            uri,
          },
        ]),
      );
    });
    expect(container.textContent).toBe("denied");

    await act(async () => {
      resolveListRecords({ data: { records: [{ uri, value: permission }] } });
      await vi.advanceTimersByTimeAsync(0);
    });

    expect(container.textContent).toBe("denied");
  });
});
