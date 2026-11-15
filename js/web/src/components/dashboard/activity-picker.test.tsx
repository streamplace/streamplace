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
  expect(selected).toEqual({
    $type: "place.stream.defs#activityGame",
    uri,
    name: "Cooking Simulator",
  });
  await act(async () => release());
  expect(container.textContent).not.toContain("Old result");
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
  expect(selected).toEqual({
    $type: "place.stream.defs#activityGame",
    uri,
    name: "Cooking Simulator",
  });
  selected = undefined;
  await search(uri);
  expect(selected).toEqual({
    $type: "place.stream.defs#activityGame",
    uri,
    name: "Cooking Simulator",
  });
});

const gameResult = {
  $type: "games.gamesgamesgamesgames.defs#gameSummaryView",
  uri,
  name: "Cooking Simulator",
};

function response(results: object[]) {
  return new Response(JSON.stringify({ results }), {
    headers: { "Content-Type": "application/json" },
  });
}

async function renderPicker() {
  useStore.setState({ url: "https://node.example", anonPDSAgent: null });
  const onChange = vi.fn();
  await act(async () =>
    root.render(
      <I18nextProvider i18n={i18n}>
        <ActivityPicker value={undefined} onChange={onChange} />
      </I18nextProvider>,
    ),
  );
  return { onChange, input: container.querySelector("input")! };
}

async function changeInput(input: HTMLInputElement, value: string) {
  await act(async () => {
    Object.getOwnPropertyDescriptor(
      HTMLInputElement.prototype,
      "value",
    )!.set!.call(input, value);
    input.dispatchEvent(new Event("input", { bubbles: true }));
  });
}

it.each(["paste", "enter"])(
  "selects one game on %s without waiting for the debounce",
  async (action) => {
    vi.stubGlobal("fetch", async () => response([gameResult]));
    const { input, onChange } = await renderPicker();
    await changeInput(input, action === "paste" ? cartridgeURL : "cooking");
    if (action === "enter") {
      await act(async () =>
        input.dispatchEvent(
          new KeyboardEvent("keydown", { key: "Enter", bubbles: true }),
        ),
      );
    }
    await act(async () => vi.advanceTimersByTimeAsync(0));
    expect(onChange).toHaveBeenCalledWith({
      $type: "place.stream.defs#activityGame",
      uri,
      name: "Cooking Simulator",
    });
  },
);

it("leaves multiple matches for the user to choose", async () => {
  vi.stubGlobal("fetch", async () =>
    response([
      { ...gameResult, name: "First game" },
      { ...gameResult, uri: `${uri}2`, name: "Second game" },
    ]),
  );
  const { input, onChange } = await renderPicker();
  await changeInput(input, igdbURL);
  await act(async () => vi.advanceTimersByTimeAsync(0));
  expect(onChange).not.toHaveBeenCalled();
  expect(container.textContent).toContain("First game");
  expect(container.textContent).toContain("Second game");
});

it("does not select a pasted link after the query changes", async () => {
  let release!: () => void;
  const pending = new Promise<void>((resolve) => {
    release = resolve;
  });
  vi.stubGlobal("fetch", async () => {
    await pending;
    return response([gameResult]);
  });
  const { input, onChange } = await renderPicker();
  await changeInput(input, cartridgeURL);
  await act(async () => vi.advanceTimersByTimeAsync(0));
  await changeInput(input, "another game");
  await act(async () => release());
  expect(onChange).not.toHaveBeenCalled();
});

it("waits for enter when a game link is typed character by character", async () => {
  vi.stubGlobal("fetch", async () => response([gameResult]));
  const { input, onChange } = await renderPicker();
  for (let i = 1; i <= cartridgeURL.length; i++) {
    await changeInput(input, cartridgeURL.slice(0, i));
  }
  await act(async () => vi.advanceTimersByTimeAsync(300));
  expect(onChange).not.toHaveBeenCalled();
  await act(async () =>
    input.dispatchEvent(
      new KeyboardEvent("keydown", { key: "Enter", bubbles: true }),
    ),
  );
  expect(onChange).toHaveBeenCalledWith({
    $type: "place.stream.defs#activityGame",
    uri,
    name: "Cooking Simulator",
  });
});
