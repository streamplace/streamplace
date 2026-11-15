import { useStore } from "@/lib/store";
import { createInstance } from "i18next";
import { act } from "react";
import { createRoot, type Root } from "react-dom/client";
import { I18nextProvider, initReactI18next } from "react-i18next";
import { StreamplaceAgent } from "streamplace";
import { afterEach, beforeAll, beforeEach, expect, it, vi } from "vitest";
import { ActivityPicker } from "./activity-picker";

const i18n = createInstance();
const previousStore = useStore.getState();
const uri =
  "at://did:web:gamesgamesgamesgames.games/games.gamesgamesgamesgames.game/3mghjjzsvqv2d";
const cartridgeURL = "https://cartridge.dev/game/cooking-simulator";
const igdbURL = "https://www.igdb.com/games/cooking-simulator";

vi.hoisted(() => {
  Object.defineProperty(navigator, "locks", {
    configurable: true,
    value: {
      request: () => {
        throw new Error("Unexpected lock request");
      },
    },
  });
});

beforeAll(async () => {
  await i18n.use(initReactI18next).init({ lng: "en", resources: { en: {} } });
});

let root: Root;
let container: HTMLDivElement;
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
  useStore.setState(previousStore, true);
  vi.useRealTimers();
  vi.unstubAllGlobals();
});

async function search(query: string) {
  const input = container.querySelector("input")!;
  await act(async () => {
    Object.getOwnPropertyDescriptor(
      HTMLInputElement.prototype,
      "value",
    )!.set!.call(input, query);
    input.dispatchEvent(new Event("input", { bubbles: true }));
    await vi.advanceTimersByTimeAsync(300);
  });
}

it("keeps the pasted game's result when an older search finishes late", async () => {
  let release!: () => void;
  const slowSearch = new Promise<void>((resolve) => {
    release = resolve;
  });
  const agent = new StreamplaceAgent(async (path) => {
    const query = new URL(path, "https://node.example").searchParams.get("q");
    if (query === "old search") await slowSearch;
    const name = query === "old search" ? "Old result" : "Cooking Simulator";
    return new Response(
      JSON.stringify({
        results: [
          {
            $type: "games.gamesgamesgamesgames.defs#gameSummaryView",
            uri,
            name,
          },
        ],
      }),
      { headers: { "Content-Type": "application/json" } },
    );
  });
  vi.stubGlobal(
    "fetch",
    async (input: RequestInfo | URL, init?: RequestInit) => {
      const url = new URL(input instanceof Request ? input.url : String(input));
      if (url.origin !== "https://node.example")
        return new Response(null, { status: 501 });
      return agent.fetchHandler(url.pathname + url.search, init ?? {});
    },
  );
  useStore.setState({ url: "https://node.example", anonPDSAgent: null });
  let selected: unknown;
  await act(async () =>
    root.render(
      <I18nextProvider i18n={i18n}>
        <ActivityPicker
          value={undefined}
          onChange={(value) => {
            selected = value;
          }}
        />
      </I18nextProvider>,
    ),
  );
  await search("old search");
  await search(cartridgeURL);
  expect(container.textContent).toContain("Cooking Simulator");
  await act(async () => release());
  expect(container.textContent).toContain("Cooking Simulator");
  expect(container.textContent).not.toContain("Old result");
  const result = Array.from(container.querySelectorAll("button")).find((el) =>
    el.textContent?.includes("Cooking Simulator"),
  );
  await act(async () => result?.click());
  expect(selected).toEqual({
    $type: "place.stream.defs#activityGame",
    uri,
    name: "Cooking Simulator",
  });
});

it("distinguishes an unavailable catalog from no result and accepts a game AT URI", async () => {
  const agent = new StreamplaceAgent(async (path) => {
    const query = new URL(path, "https://node.example").searchParams.get("q");
    if (query === "unavailable") {
      return new Response(
        JSON.stringify({ error: "UpstreamFailure", message: "Unavailable" }),
        { status: 502, headers: { "Content-Type": "application/json" } },
      );
    }
    const results =
      query === uri || query === igdbURL
        ? [
            {
              $type: "games.gamesgamesgamesgames.defs#gameSummaryView",
              uri,
              name: "Cooking Simulator",
            },
          ]
        : [];
    return new Response(JSON.stringify({ results }), {
      headers: { "Content-Type": "application/json" },
    });
  });
  vi.stubGlobal(
    "fetch",
    async (input: RequestInfo | URL, init?: RequestInit) => {
      const url = new URL(input instanceof Request ? input.url : String(input));
      if (url.origin !== "https://node.example")
        return new Response(null, { status: 501 });
      return agent.fetchHandler(url.pathname + url.search, init ?? {});
    },
  );
  useStore.setState({ url: "https://node.example", anonPDSAgent: null });
  let selected: unknown;
  await act(async () =>
    root.render(
      <I18nextProvider i18n={i18n}>
        <ActivityPicker
          value={undefined}
          onChange={(value) => {
            selected = value;
          }}
        />
      </I18nextProvider>,
    ),
  );
  await search("unavailable");
  expect(container.textContent).toContain("Search failed");
  await search("missing");
  expect(container.textContent).toContain("No games found");
  expect(container.textContent).not.toContain("Search failed");
  await search(igdbURL);
  expect(container.textContent).toContain("Cooking Simulator");
  await search(uri);
  const result = Array.from(container.querySelectorAll("button")).find((el) =>
    el.textContent?.includes("Cooking Simulator"),
  );
  await act(async () => result?.click());
  expect(selected).toEqual({
    $type: "place.stream.defs#activityGame",
    uri,
    name: "Cooking Simulator",
  });
});
