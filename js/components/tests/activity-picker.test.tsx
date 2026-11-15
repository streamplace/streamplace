import { act, createElement, type ReactNode } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeEach, expect, it, vi } from "vitest";
import ActivityPicker from "../../app/components/activity-picker";

function View({ children }: { children?: ReactNode }) {
  return createElement("div", null, children);
}
vi.mock("react-native", () => ({
  View,
  ScrollView: View,
  Pressable: ({ children, onPress, disabled }: any) =>
    createElement("button", { onClick: onPress, disabled }, children),
  Platform: { OS: "android", select: (value: any) => value.android },
}));
vi.mock("@streamplace/components", async () => ({
  zero: await import("../src/lib/theme/atoms"),
  useTheme: () => ({
    theme: {
      colors: {},
      spacing: { 16: 64, 64: 256 },
      touchTargets: { minimum: 44 },
    },
  }),
  DropdownMenu: View,
  DropdownMenuTrigger: View,
  ResponsiveDropdownMenuContent: ({ children, onContentScroll }: any) =>
    createElement(
      "div",
      {
        "data-testid": "native-menu",
        onScroll: (e: any) =>
          onContentScroll?.({
            nativeEvent: {
              contentOffset: { y: e.currentTarget.scrollTop },
              contentSize: { height: e.currentTarget.scrollHeight },
              layoutMeasurement: { height: e.currentTarget.clientHeight },
            },
          }),
      },
      children,
    ),
  DropdownMenuItem: ({ children, onPress }: any) =>
    createElement("button", { onClick: onPress }, children),
  Text: ({ children, numberOfLines }: any) =>
    createElement("span", { "data-lines": numberOfLines }, children),
  Input: ({ value, onChangeText, placeholder }: any) =>
    createElement("input", {
      value,
      placeholder,
      onChange: (e: any) => onChangeText(e.target.value),
    }),
}));
vi.mock(
  "@streamplace/components/src/streamplace-store/streamplace-store",
  () => ({
    useStreamplaceStore: (selector: any) =>
      selector({ gamesEnabled: true, url: "https://node.example" }),
  }),
);
vi.mock("expo-image", () => ({ Image: () => null }));
vi.mock("lucide-react-native", () => ({ X: () => null }));
let container: HTMLDivElement;
let root: Root;
beforeEach(() => {
  vi.stubGlobal("IS_REACT_ACT_ENVIRONMENT", true);
  vi.useFakeTimers();
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

it("loads more games on scroll and allows two-line titles", async () => {
  const requests: URL[] = [];
  vi.stubGlobal("fetch", async (input: RequestInfo | URL) => {
    const url = new URL(input instanceof Request ? input.url : String(input));
    requests.push(url);
    const cursor = url.searchParams.get("cursor");
    return new Response(
      JSON.stringify({
        results: [
          {
            $type: "games.gamesgamesgamesgames.defs#gameSummaryView",
            uri: `at://did:plc:ho26ynsdw2ey7l56xx4owjzd/games.gamesgamesgamesgames.game/${cursor ? "second" : "first"}`,
            name: cursor ? "Second page" : "Long Zelda title",
          },
        ],
        ...(!cursor ? { cursor: "page-2" } : {}),
      }),
      { headers: { "Content-Type": "application/json" } },
    );
  });
  await act(async () =>
    root.render(<ActivityPicker value={undefined} onChange={() => {}} />),
  );
  const input = container.querySelector("input")!;
  await act(async () => {
    Object.getOwnPropertyDescriptor(
      HTMLInputElement.prototype,
      "value",
    )!.set!.call(input, "zelda");
    input.dispatchEvent(new Event("input", { bubbles: true }));
    await vi.advanceTimersByTimeAsync(300);
  });
  expect(requests[0].origin).toBe("https://node.example");
  expect(requests[0].searchParams.getAll("types")).toEqual(["game"]);
  expect(requests[0].searchParams.get("limit")).toBe("15");
  expect(container.querySelector('[data-lines="2"]')?.textContent).toContain(
    "Long Zelda title",
  );
  const list = container.querySelector('[data-testid="native-menu"]')!;
  Object.defineProperties(list, {
    scrollTop: { value: 700 },
    scrollHeight: { value: 1000 },
    clientHeight: { value: 300 },
  });
  await act(async () =>
    list.dispatchEvent(new Event("scroll", { bubbles: true })),
  );
  expect(requests[1].searchParams.get("cursor")).toBe("page-2");
  expect(container.textContent).toContain("Second page");
  expect(container.textContent).toContain("Long Zelda title");
});
