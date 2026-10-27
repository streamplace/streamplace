import { makeLivestreamStore, type LivestreamStore } from "@streamplace/core";
import { act, type ReactNode } from "react";
import { createRoot } from "react-dom/client";
import { afterEach, beforeAll, beforeEach, expect, it, vi } from "vitest";
import LiveDashboard from "../../app/src/screens/live-dashboard";
import { LivestreamContext } from "../src/livestream-store/context";
import { getStoreFromContext } from "../src/livestream-store/use-store";

const session = vi.hoisted(() => ({
  ready: true,
  restoring: false,
  focused: true,
  did: "did:plc:creator",
  componentDid: undefined as string | undefined,
  node: "https://node.example",
  login: vi.fn(),
  warnings: vi.spyOn(console, "warn").mockImplementation(() => {}),
}));

beforeAll(() => {
  for (const [message] of session.warnings.mock.calls) {
    expect(message).toBe(
      "[@atproto/lex-data]: Uint8Array.fromBase64 / Uint8Array.prototype.toBase64 not available in this environment. Falling back to ponyfill implementation.",
    );
  }
  session.warnings.mockRestore();
});

// Auth/navigation inputs and platform presentation are boundaries here. The
// dashboard host, livestream provider, store, and context ownership are real.
vi.mock("@react-navigation/native", () => ({
  useRoute: () => ({
    name: "Creator",
    params: { screen: "CreatorControlPanel" },
  }),
  useIsFocused: () => session.focused,
}));
vi.mock("@streamplace/components", async () => ({
  ...(await import("../src/livestream-provider")),
  PlayerProvider: ({ children }: { children: ReactNode }) => children,
  useUrl: () => session.node,
  useStreamplaceStore: (selector: (state: any) => unknown) =>
    selector({
      oauthSession: session.restoring
        ? undefined
        : session.did
          ? { did: session.componentDid ?? session.did }
          : null,
    }),
}));
vi.mock("store", () => ({
  useStore: (selector: (state: any) => unknown) =>
    selector({ openLoginModal: session.login }),
}));
vi.mock("store/hooks", () => ({
  useIsReady: () => session.ready,
  useUserProfile: () => (session.did ? { did: session.did } : null),
}));
vi.mock("hooks/useLiveUser", () => ({ useLiveUser: () => false }));
vi.mock("components/loading/loading", () => ({ default: () => "Loading" }));
vi.mock("components/live-dashboard/bento-grid", () => ({
  default: () => null,
}));
vi.mock("../src/hooks", () => ({ useAvatars: () => ({}) }));
vi.mock("../src/streamplace-store", () => ({
  useDID: () => session.did,
  usePDSAgent: () => null,
}));
vi.mock("../src/streamplace-store/xrpc", () => ({ usePDSAgent: () => null }));
vi.mock("../src/lib/stream-notifications", () => ({
  StreamNotifications: { pinnedCommentDismiss() {} },
}));
vi.mock("../src/livestream-provider/websocket", () => ({
  useLivestreamWebsocket() {},
}));

let root: ReturnType<typeof createRoot>;
let container: HTMLDivElement;
let current: LivestreamStore;
let outer: LivestreamStore;

function Content({ section }: { section: string }) {
  current = getStoreFromContext();
  return <span>{section}</span>;
}

async function render(section = "Stream") {
  await act(async () => {
    root.render(
      <LivestreamContext.Provider value={{ store: outer }}>
        <LiveDashboard>
          <Content section={section} />
        </LiveDashboard>
      </LivestreamContext.Provider>,
    );
  });
}

beforeEach(() => {
  vi.stubGlobal("IS_REACT_ACT_ENVIRONMENT", true);
  Object.assign(session, {
    ready: true,
    restoring: false,
    focused: true,
    did: "did:plc:creator",
    componentDid: undefined,
    node: "https://node.example",
  });
  session.login.mockClear();
  outer = makeLivestreamStore();
  outer.setState({
    chatDraft: "Another streamer's draft",
    streamKey: "outer-key",
  });
  container = document.createElement("div");
  root = createRoot(container);
});

afterEach(async () => {
  await act(async () => root.unmount());
  vi.unstubAllGlobals();
});

it("owns one isolated store across creator section changes", async () => {
  await render();
  expect(current).not.toBe(outer);
  current.setState({ chatDraft: "Unsent creator message" });
  const first = current;
  await render("Chat");
  expect(current).toBe(first);
  expect(current.getState().chatDraft).toBe("Unsent creator message");
  expect(current.getState().streamKey).toBeNull();
});

it("does not retain the private store on the browser global", async () => {
  await render();
  expect((window as any).livestreamStore).toBeUndefined();
});

it.each(["did", "node"] as const)(
  "resets drafts and secrets when %s changes",
  async (field) => {
    await render();
    current.setState({ chatDraft: "Private draft", streamKey: "private-key" });
    const first = current;
    session[field] =
      field === "did" ? "did:plc:other" : "https://other.example";
    await render();
    expect(current).not.toBe(first);
    expect(current.getState().chatDraft).toBe("");
    expect(current.getState().streamKey).toBeNull();
  },
);

it("releases the creator session on blur and starts fresh on return", async () => {
  await render();
  const first = current;
  current.setState({ chatDraft: "Private draft", streamKey: "private-key" });
  session.focused = false;
  await render();
  expect(container.textContent).not.toContain("Stream");
  session.focused = true;
  await render();
  expect(current).not.toBe(first);
  expect(current.getState().streamKey).toBeNull();
});

it("waits until the component session matches the app's new account", async () => {
  await render();
  const first = current;
  session.componentDid = session.did;
  session.did = "did:plc:other";
  await render();
  expect(container.textContent).toBe("Loading");
  session.componentDid = session.did;
  await render();
  expect(current).not.toBe(first);
  expect(container.textContent).toBe("Stream");
});

it("waits for restoration, then offers login with the creator return destination", async () => {
  session.did = "";
  session.restoring = true;
  await render();
  expect(session.login).not.toHaveBeenCalled();
  session.restoring = false;
  await render();
  expect(session.login.mock.calls).toEqual([
    [
      {
        name: "Creator",
        params: { screen: "CreatorControlPanel" },
      },
    ],
  ]);
  session.focused = false;
  await render();
  expect(session.login).toHaveBeenCalledTimes(1);
});
