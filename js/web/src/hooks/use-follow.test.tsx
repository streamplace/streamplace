import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { act } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { useFollow } from "./use-follow";

const { mockUseSession } = vi.hoisted(() => ({ mockUseSession: vi.fn() }));
vi.mock("@/lib/session", () => ({ useSession: mockUseSession }));
vi.mock("@/lib/store", () => ({
  useStore: (selector: (state: { url: string }) => unknown) =>
    selector({ url: "https://node.example" }),
}));

const VIEWER = "did:plc:viewer";
const STREAMER = "did:plc:streamer";
const FOLLOW_URI = `at://${VIEWER}/app.bsky.graph.follow/record`;

function response(uri: string | null) {
  return new Response(JSON.stringify(uri ? { follow: { uri } } : {}));
}

describe("useFollow", () => {
  let container: HTMLDivElement;
  let root: Root;
  let client: QueryClient;
  let current: ReturnType<typeof useFollow>;
  let fetchMock: ReturnType<typeof vi.fn>;
  let agent: {
    follow: ReturnType<typeof vi.fn>;
    deleteFollow: ReturnType<typeof vi.fn>;
  };

  function Probe({ subject }: { subject?: string }) {
    current = useFollow(subject);
    return <span>{current.following ? "Following" : "Follow"}</span>;
  }

  async function render(subject: string | undefined = STREAMER) {
    await act(async () => {
      root.render(
        <QueryClientProvider client={client}>
          <Probe subject={subject} />
        </QueryClientProvider>,
      );
    });
  }

  async function waitForLoaded() {
    await act(async () => {
      await vi.advanceTimersByTimeAsync(0);
    });
    expect(current.loading).toBe(false);
  }

  beforeEach(() => {
    vi.useFakeTimers();
    vi.stubGlobal("IS_REACT_ACT_ENVIRONMENT", true);
    fetchMock = vi.fn().mockResolvedValue(response(null));
    vi.stubGlobal("fetch", fetchMock);
    agent = {
      follow: vi.fn().mockResolvedValue({ uri: FOLLOW_URI }),
      deleteFollow: vi.fn().mockResolvedValue(undefined),
    };
    mockUseSession.mockReturnValue({ did: VIEWER, pdsAgent: agent });
    client = new QueryClient({
      defaultOptions: { queries: { retry: false, gcTime: Infinity } },
    });
    container = document.createElement("div");
    document.body.appendChild(container);
    root = createRoot(container);
  });

  afterEach(async () => {
    await act(async () => root.unmount());
    client.clear();
    container.remove();
    mockUseSession.mockReset();
    vi.unstubAllGlobals();
    vi.useRealTimers();
  });

  it("loads an existing follow record for the viewer and streamer", async () => {
    fetchMock.mockResolvedValue(response(FOLLOW_URI));
    await render();
    await waitForLoaded();

    expect(container.textContent).toBe("Following");
    const url = new URL(fetchMock.mock.calls[0][0]);
    expect(url.origin).toBe("https://node.example");
    expect(url.searchParams.get("userDID")).toBe(VIEWER);
    expect(url.searchParams.get("subjectDID")).toBe(STREAMER);
    await act(async () => current.toggle());
    expect(agent.deleteFollow).toHaveBeenCalledWith(FOLLOW_URI);
    expect(container.textContent).toBe("Follow");
  });

  it("keeps the returned URI after following so the next click unfollows", async () => {
    await render();
    await waitForLoaded();
    expect(container.textContent).toBe("Follow");

    await act(async () => current.toggle());
    expect(agent.follow).toHaveBeenCalledWith(STREAMER);
    expect(container.textContent).toBe("Following");
    await act(async () => current.toggle());
    expect(agent.deleteFollow).toHaveBeenCalledWith(FOLLOW_URI);
    expect(container.textContent).toBe("Follow");
  });

  it("keeps a successful follow while the node index is still stale", async () => {
    fetchMock
      .mockResolvedValueOnce(response(null))
      .mockResolvedValueOnce(response(null));
    await render();
    await waitForLoaded();

    await act(async () => current.toggle());
    expect(container.textContent).toBe("Following");

    await render(undefined);
    await render();
    await waitForLoaded();

    expect(container.textContent).toBe("Following");
    await act(async () => current.toggle());
    expect(agent.deleteFollow).toHaveBeenCalledWith(FOLLOW_URI);
  });

  it("does not allow a follow before the lookup completes", async () => {
    fetchMock.mockReturnValue(new Promise(() => {}));
    await render();
    expect(current.loading).toBe(true);
    await act(async () => current.toggle());
    expect(agent.follow).not.toHaveBeenCalled();
  });

  it("keeps lookup errors distinct from not following", async () => {
    fetchMock.mockResolvedValue(new Response("failed", { status: 500 }));
    await render();
    await waitForLoaded();
    expect(current.error).toBeInstanceOf(Error);
    await act(async () => current.toggle());
    expect(agent.follow).not.toHaveBeenCalled();
  });

  it("preserves the record when unfollow fails", async () => {
    fetchMock.mockResolvedValue(response(FOLLOW_URI));
    const error = new Error("write failed");
    agent.deleteFollow.mockRejectedValue(error);
    await render();
    await waitForLoaded();
    await act(async () => {
      await expect(current.toggle()).rejects.toBe(error);
    });
    expect(container.textContent).toBe("Following");
    expect(current.loading).toBe(false);
  });

  it("preserves not-following state when follow fails", async () => {
    const error = new Error("write failed");
    agent.follow.mockRejectedValue(error);
    await render();
    await waitForLoaded();
    await act(async () => {
      await expect(current.toggle()).rejects.toBe(error);
    });
    expect(container.textContent).toBe("Follow");
    expect(current.loading).toBe(false);
  });

  it("blocks another toggle while a write is pending", async () => {
    let resolve!: (value: { uri: string }) => void;
    agent.follow.mockImplementation(
      () =>
        new Promise<{ uri: string }>((done) => {
          resolve = done;
        }),
    );
    await render();
    await waitForLoaded();
    let write!: Promise<void>;
    await act(async () => {
      write = current.toggle();
    });
    expect(current.loading).toBe(true);
    await act(async () => current.toggle());
    expect(agent.follow).toHaveBeenCalledTimes(1);
    await act(async () => {
      resolve({ uri: FOLLOW_URI });
      await write;
    });
    expect(container.textContent).toBe("Following");
  });

  it("does not apply a late follow write to a different streamer", async () => {
    fetchMock.mockImplementation(async () => response(null));
    let resolve!: (value: { uri: string }) => void;
    agent.follow.mockImplementation(
      () =>
        new Promise<{ uri: string }>((done) => {
          resolve = done;
        }),
    );
    await render();
    await waitForLoaded();
    let write!: Promise<void>;
    await act(async () => {
      write = current.toggle();
    });
    await render("did:plc:other");
    await act(async () => {
      resolve({ uri: FOLLOW_URI });
      await write;
      await vi.runAllTimersAsync();
    });
    expect(container.textContent).toBe("Follow");
    expect(
      client.getQueryData(["follow", "https://node.example", VIEWER, STREAMER]),
    ).toMatchObject({ uri: FOLLOW_URI, reconcileUntil: expect.any(Number) });
  });

  it("stops reconciling after the index window expires", async () => {
    fetchMock.mockResolvedValue(response(null));
    await render();
    await waitForLoaded();
    await act(async () => current.toggle());

    await act(async () => {
      await vi.advanceTimersByTimeAsync(60_000);
    });

    expect(fetchMock).toHaveBeenCalledTimes(7);
    await act(async () => {
      await vi.advanceTimersByTimeAsync(120_000);
    });
    expect(fetchMock).toHaveBeenCalledTimes(7);
  });

  it("allows changes after a background reconciliation error", async () => {
    fetchMock.mockResolvedValueOnce(response(null));
    await render();
    await waitForLoaded();
    await act(async () => current.toggle());

    fetchMock.mockRejectedValueOnce(new Error("node unavailable"));
    await act(async () => {
      await client.refetchQueries({
        queryKey: ["follow", "https://node.example", VIEWER, STREAMER],
      });
    });
    expect(fetchMock).toHaveBeenCalledTimes(2);

    await act(async () => current.toggle());
    expect(agent.deleteFollow).toHaveBeenCalledWith(FOLLOW_URI);
    expect(container.textContent).toBe("Follow");
  });

  it("does not apply a late lookup to a different streamer", async () => {
    let resolve!: (value: Response) => void;
    fetchMock.mockImplementationOnce(
      () =>
        new Promise<Response>((done) => {
          resolve = done;
        }),
    );
    await render();
    await render("did:plc:other");
    await waitForLoaded();
    await act(async () => resolve(response(FOLLOW_URI)));
    expect(container.textContent).toBe("Follow");
  });

  it("does not reuse another viewer's follow state", async () => {
    fetchMock.mockResolvedValueOnce(response(FOLLOW_URI));
    await render();
    await waitForLoaded();
    mockUseSession.mockReturnValue({
      did: "did:plc:other-viewer",
      pdsAgent: agent,
    });
    await render();
    await waitForLoaded();
    expect(container.textContent).toBe("Follow");
    const url = new URL(fetchMock.mock.calls.at(-1)![0]);
    expect(url.searchParams.get("userDID")).toBe("did:plc:other-viewer");
  });

  it("skips lookups and writes when logged out or viewing yourself", async () => {
    mockUseSession.mockReturnValue({ did: null, pdsAgent: null });
    await render();
    await act(async () => current.toggle());
    mockUseSession.mockReturnValue({ did: VIEWER, pdsAgent: agent });
    await render(VIEWER);
    await act(async () => current.toggle());
    expect(fetchMock).not.toHaveBeenCalled();
    expect(agent.follow).not.toHaveBeenCalled();
  });
});
