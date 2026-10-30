import { act, createElement } from "react";
import { createRoot } from "react-dom/client";
import { afterEach, beforeEach, expect, it, vi } from "vitest";
import FollowButton from "../../app/components/follow-button";

const fixture = vi.hoisted(() => ({
  following: new Set<string>(),
  fail: false,
  release: null as (() => void) | null,
  pending: false,
  toasts: [] as string[],
}));
function Button({
  children,
  onPress,
  accessibilityLabel,
  disabled,
  loading,
  leftIcon,
}: any) {
  return (
    <button
      aria-label={accessibilityLabel}
      disabled={disabled || loading}
      onClick={onPress}
    >
      {leftIcon}
      {children}
    </button>
  );
}
vi.mock("react-native", () => ({
  View: ({ children }: any) => <div>{children}</div>,
}));
vi.mock("lucide-react-native", () => ({
  Plus: () => <span>Plus</span>,
  Check: () => <span>Check</span>,
}));
vi.mock("@streamplace/components", () => ({
  Button,
  IconButton: Button,
  Icon: ({ icon }: any) => createElement(icon),
  Loader: () => <span>Loading</span>,
  Text: ({ children }: any) => <span>{children}</span>,
  useTheme: () => ({
    theme: { colors: { danger: "red", text2: "gray" }, spacing: { 4: 16 } },
  }),
  useTranslation: () => ({
    t: (key: string) =>
      ({
        follow: "Follow",
        unfollow: "Unfollow",
        following: "Following",
        loading: "Loading...",
        "follow-update-failed": "Couldn’t update follow",
      })[key] ?? key,
  }),
  toast: {
    show: (title: string, description: string) =>
      fixture.toasts.push(`${title}: ${description}`),
  },
  zero: { gap: { all: { 2: {} } }, ml: { 2: {} } },
}));
vi.mock("store/hooks", () => ({
  useStreamplaceUrl: () => "https://node.example",
}));
vi.mock("store", () => ({
  useStore: (select: any) =>
    select({
      followUser: async (did: string) => {
        if (fixture.pending)
          await new Promise<void>((resolve) => {
            fixture.release = resolve;
          });
        if (fixture.fail) throw new Error("Request rejected");
        fixture.following.add(did);
      },
      unfollowUser: async (did: string) => {
        if (fixture.fail) throw new Error("Request rejected");
        fixture.following.delete(did);
      },
    }),
}));
let root: ReturnType<typeof createRoot>;
let container: HTMLDivElement;
beforeEach(() => {
  vi.stubGlobal("IS_REACT_ACT_ENVIRONMENT", true);
  fixture.following.clear();
  fixture.fail = false;
  fixture.pending = false;
  fixture.release = null;
  fixture.toasts = [];
  vi.stubGlobal("fetch", async () => ({
    ok: true,
    json: async () => ({
      follow: fixture.following.has("creator")
        ? { uri: "at://viewer/app.bsky.graph.follow/test" }
        : null,
    }),
  }));
  container = document.createElement("div");
  root = createRoot(container);
});
afterEach(async () => {
  await act(async () => root.unmount());
  vi.unstubAllGlobals();
});
async function render(
  currentUserDID: string | undefined = "viewer",
  iconOnly = true,
) {
  await act(async () =>
    root.render(
      <FollowButton
        streamerDID="creator"
        currentUserDID={currentUserDID}
        iconOnly={iconOnly}
      />,
    ),
  );
}

it("uses an accessible plus and check while preserving real follow state", async () => {
  await render();
  let button = container.querySelector("button")!;
  expect(button.getAttribute("aria-label")).toBe("Follow");
  expect(button.textContent).toBe("Plus");
  await act(async () => button.click());
  expect(fixture.following.has("creator")).toBe(true);
  button = container.querySelector("button")!;
  expect(button.getAttribute("aria-label")).toBe("Unfollow");
  expect(button.textContent).toBe("Check");
  await act(async () => button.click());
  expect(fixture.following.has("creator")).toBe(false);
  expect(container.querySelector("button")!.textContent).toBe("Plus");
});

it.each([undefined, "creator"])(
  "hides the follow action for %s",
  async (currentUser) => {
    await act(async () =>
      root.render(
        <FollowButton
          streamerDID="creator"
          currentUserDID={currentUser}
          iconOnly
        />,
      ),
    );
    expect(container.querySelector("button")).toBeNull();
  },
);

it("keeps the full button's text presentation", async () => {
  fixture.following.add("creator");
  await render("viewer", false);
  expect(container.querySelector("button")!.textContent).toBe("Following");
});

it.each([false, true])(
  "restores the previous follow state (%s) after a failed update",
  async (following) => {
    if (following) fixture.following.add("creator");
    await render();
    fixture.fail = true;
    await act(async () => container.querySelector("button")!.click());
    expect(fixture.following.has("creator")).toBe(following);
    expect(container.querySelector("button")!.textContent).toBe(
      following ? "Check" : "Plus",
    );
    expect(fixture.toasts.join(" ")).toContain("Request rejected");
  },
);

it("does not let another press reverse an update that is still pending", async () => {
  await render();
  fixture.pending = true;
  await act(async () => container.querySelector("button")!.click());
  expect(container.querySelector("button")!.disabled).toBe(true);
  await act(async () => container.querySelector("button")!.click());
  await act(async () => fixture.release!());
  expect(fixture.following.has("creator")).toBe(true);
  expect(container.querySelector("button")!.textContent).toBe("Check");
  expect(container.querySelector("button")!.disabled).toBe(false);
});
