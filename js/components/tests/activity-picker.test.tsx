import { act, createElement, type ReactNode } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeEach, expect, it, vi } from "vitest";
import ActivityPicker from "../../app/components/activity-picker";

function View({ children }: { children?: ReactNode }) {
  return createElement("div", null, children);
}
vi.mock("react-native", () => ({
  View,
  Pressable: ({ children, onPress }: any) =>
    createElement("button", { onClick: onPress }, children),
  Platform: { OS: "android", select: (value: any) => value.android },
}));
vi.mock("@streamplace/components", async () => ({
  zero: await import("../src/lib/theme/atoms"),
  useTheme: () => ({ theme: { colors: {} } }),
  DropdownMenu: View,
  DropdownMenuTrigger: View,
  ResponsiveDropdownMenuContent: View,
  DropdownMenuItem: ({ children, onPress }: any) =>
    createElement("button", { onClick: onPress }, children),
  Text: View,
  Input: ({ value, onChangeText, onSubmitEditing, placeholder }: any) =>
    createElement("input", {
      value,
      placeholder,
      onChange: (e: any) => onChangeText(e.target.value),
      onKeyDown: (e: any) => e.key === "Enter" && onSubmitEditing?.(),
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

const uri =
  "at://did:web:gamesgamesgamesgames.games/games.gamesgamesgamesgames.game/3mghjjzsvqv2d";
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

it.each([
  uri,
  "https://cartridge.dev/game/cooking-simulator",
  "https://www.igdb.com/games/cooking-simulator",
])("selects a game from pasted %s", async (query) => {
  vi.stubGlobal("fetch", async (input: RequestInfo | URL) => {
    const url = new URL(input instanceof Request ? input.url : String(input));
    if (url.origin !== "https://node.example")
      return new Response(null, { status: 501 });
    const inputQuery = url.searchParams.get("q");
    if (url.pathname.endsWith(".getGame")) {
      return new Response(JSON.stringify({ uri, name: "Cooking Simulator" }), {
        headers: { "Content-Type": "application/json" },
      });
    }
    return new Response(
      JSON.stringify({
        results:
          inputQuery === "cooking simulator" && query.includes("igdb.com")
            ? [
                {
                  $type: "games.gamesgamesgamesgames.defs#gameSummaryView",
                  uri,
                  name: "Cooking Simulator",
                },
              ]
            : [],
      }),
      { headers: { "Content-Type": "application/json" } },
    );
  });
  let selected: unknown;
  await act(async () =>
    root.render(
      <ActivityPicker
        value={undefined}
        onChange={(value) => {
          selected = value;
        }}
      />,
    ),
  );
  const input = container.querySelector("input")!;
  await act(async () => {
    Object.getOwnPropertyDescriptor(
      HTMLInputElement.prototype,
      "value",
    )!.set!.call(input, query);
    input.dispatchEvent(new Event("input", { bubbles: true }));
    await vi.advanceTimersByTimeAsync(0);
  });
  expect(selected).toEqual({
    $type: "place.stream.defs#activityGame",
    uri,
    name: "Cooking Simulator",
  });
});

it("selects a single game when the search is submitted", async () => {
  vi.stubGlobal(
    "fetch",
    async () =>
      new Response(
        JSON.stringify({
          results: [
            {
              $type: "games.gamesgamesgamesgames.defs#gameSummaryView",
              uri,
              name: "Cooking Simulator",
            },
          ],
        }),
        { headers: { "Content-Type": "application/json" } },
      ),
  );
  const onChange = vi.fn();
  await act(async () =>
    root.render(<ActivityPicker value={undefined} onChange={onChange} />),
  );
  const input = container.querySelector("input")!;
  await act(async () => {
    Object.getOwnPropertyDescriptor(
      HTMLInputElement.prototype,
      "value",
    )!.set!.call(input, "cooking");
    input.dispatchEvent(new Event("input", { bubbles: true }));
  });
  await act(async () => {
    input.dispatchEvent(
      new KeyboardEvent("keydown", { key: "Enter", bubbles: true }),
    );
    await vi.advanceTimersByTimeAsync(0);
  });
  expect(onChange).toHaveBeenCalledWith({
    $type: "place.stream.defs#activityGame",
    uri,
    name: "Cooking Simulator",
  });
});
