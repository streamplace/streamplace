import { useStore } from "@/lib/store";
import { createInstance } from "i18next";
import { act } from "react";
import { createRoot, type Root } from "react-dom/client";
import { I18nextProvider, initReactI18next } from "react-i18next";
import { afterEach, beforeAll, beforeEach, expect, it, vi } from "vitest";
import { ActivityPicker } from "./activity-picker";

const i18n = createInstance();
const previousStore = useStore.getState();
vi.hoisted(() =>
  Object.defineProperty(navigator, "locks", {
    configurable: true,
    value: {
      request: () => {
        throw new Error("Unexpected lock request");
      },
    },
  }),
);
beforeAll(async () =>
  i18n.use(initReactI18next).init({ lng: "en", resources: { en: {} } }),
);
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

it("loads more game-only results on scroll and starts over for a new query", async () => {
  const requests: URL[] = [];
  vi.stubGlobal("fetch", async (input: RequestInfo | URL) => {
    const url = new URL(input instanceof Request ? input.url : String(input));
    requests.push(url);
    const cursor = url.searchParams.get("cursor");
    const q = url.searchParams.get("q");
    const name =
      q === "another"
        ? "Another game"
        : cursor
          ? "Second page game"
          : "A long Zelda game title that needs two lines";
    return new Response(
      JSON.stringify({
        results: [
          {
            $type: "games.gamesgamesgamesgames.defs#gameSummaryView",
            uri: `at://did:plc:ho26ynsdw2ey7l56xx4owjzd/games.gamesgamesgamesgames.game/${cursor ? "second" : "first"}`,
            name,
          },
        ],
        ...(q === "zelda" && !cursor ? { cursor: "page-2" } : {}),
      }),
      { headers: { "Content-Type": "application/json" } },
    );
  });
  useStore.setState({ url: "https://node.example", pdsAgent: null });
  await act(async () =>
    root.render(
      <I18nextProvider i18n={i18n}>
        <ActivityPicker value={undefined} onChange={() => {}} />
      </I18nextProvider>,
    ),
  );
  const input = container.querySelector("input")!;
  async function change(value: string) {
    await act(async () => {
      Object.getOwnPropertyDescriptor(
        HTMLInputElement.prototype,
        "value",
      )!.set!.call(input, value);
      input.dispatchEvent(new Event("input", { bubbles: true }));
      await vi.advanceTimersByTimeAsync(300);
    });
  }
  await change("zelda");
  expect(requests[0].origin).toBe("https://node.example");
  expect(requests[0].searchParams.get("limit")).toBe("15");
  expect(requests[0].searchParams.getAll("types")).toEqual(["game"]);
  expect(container.querySelector(".line-clamp-2")?.textContent).toContain(
    "A long Zelda",
  );
  const list = container.querySelector('[data-testid="game-results"]')!;
  Object.defineProperties(list, {
    scrollTop: { value: 700 },
    scrollHeight: { value: 1000 },
    clientHeight: { value: 300 },
  });
  await act(async () =>
    list.dispatchEvent(new Event("scroll", { bubbles: true })),
  );
  expect(requests[1].searchParams.get("cursor")).toBe("page-2");
  expect(container.textContent).toContain("Second page game");
  expect(container.textContent).toContain("A long Zelda");
  expect(container.textContent).not.toContain("Load more");
  await change("another");
  expect(requests.at(-1)?.searchParams.has("cursor")).toBe(false);
  expect(container.textContent).toContain("Another game");
  expect(container.textContent).not.toContain("Second page game");
  await change("zelda");
  expect(requests.at(-1)?.searchParams.has("cursor")).toBe(false);
  expect(container.textContent).toContain("A long Zelda");
});

it("ignores an older search that finishes after the query changes", async () => {
  let release!: () => void;
  const slow = new Promise<void>((resolve) => {
    release = resolve;
  });
  vi.stubGlobal("fetch", async (input: RequestInfo | URL) => {
    const url = new URL(input instanceof Request ? input.url : String(input));
    const query = url.searchParams.get("q");
    if (query === "old") await slow;
    return new Response(
      JSON.stringify({
        results: [
          {
            $type: "games.gamesgamesgamesgames.defs#gameSummaryView",
            uri: "at://did:plc:ho26ynsdw2ey7l56xx4owjzd/games.gamesgamesgamesgames.game/abc",
            name: query === "old" ? "Old result" : "New result",
          },
        ],
      }),
      { headers: { "Content-Type": "application/json" } },
    );
  });
  useStore.setState({ url: "https://node.example", pdsAgent: null });
  await act(async () =>
    root.render(
      <I18nextProvider i18n={i18n}>
        <ActivityPicker value={undefined} onChange={() => {}} />
      </I18nextProvider>,
    ),
  );
  const input = container.querySelector("input")!;
  for (const value of ["old", "new"]) {
    await act(async () => {
      Object.getOwnPropertyDescriptor(
        HTMLInputElement.prototype,
        "value",
      )!.set!.call(input, value);
      input.dispatchEvent(new Event("input", { bubbles: true }));
      await vi.advanceTimersByTimeAsync(300);
    });
  }
  expect(container.textContent).toContain("New result");
  await act(async () => release());
  expect(container.textContent).toContain("New result");
  expect(container.textContent).not.toContain("Old result");
});

it("keeps results and allows a retry if loading more fails", async () => {
  let attempts = 0;
  vi.stubGlobal("fetch", async (input: RequestInfo | URL) => {
    const url = new URL(input instanceof Request ? input.url : String(input));
    const cursor = url.searchParams.get("cursor");
    if (cursor && ++attempts === 1)
      return new Response(JSON.stringify({ error: "Unavailable" }), {
        status: 503,
        headers: { "Content-Type": "application/json" },
      });
    return new Response(
      JSON.stringify({
        results: [
          {
            $type: "games.gamesgamesgamesgames.defs#gameSummaryView",
            uri: `at://did:plc:ho26ynsdw2ey7l56xx4owjzd/games.gamesgamesgamesgames.game/${cursor ? "second" : "first"}`,
            name: cursor ? "Second game" : "First game",
          },
        ],
        ...(!cursor ? { cursor: "next" } : {}),
      }),
      { headers: { "Content-Type": "application/json" } },
    );
  });
  useStore.setState({ url: "https://node.example", pdsAgent: null });
  await act(async () =>
    root.render(
      <I18nextProvider i18n={i18n}>
        <ActivityPicker value={undefined} onChange={() => {}} />
      </I18nextProvider>,
    ),
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
  const list = container.querySelector('[data-testid="game-results"]')!;
  Object.defineProperties(list, {
    scrollTop: { value: 700 },
    scrollHeight: { value: 1000 },
    clientHeight: { value: 300 },
  });
  await act(async () =>
    list.dispatchEvent(new Event("scroll", { bubbles: true })),
  );
  expect(container.textContent).toContain("First game");
  expect(container.textContent).toContain("Try again");
  await act(async () =>
    list.dispatchEvent(new Event("scroll", { bubbles: true })),
  );
  expect(attempts).toBe(1);
  const retry = Array.from(container.querySelectorAll("button")).find((el) =>
    el.textContent?.includes("Try again"),
  )!;
  await act(async () => retry.click());
  expect(attempts).toBe(2);
  expect(container.textContent).toContain("First game");
  expect(container.textContent).toContain("Second game");
});

it("does not append an old page after the query changes", async () => {
  let release!: () => void;
  const pending = new Promise<void>((resolve) => {
    release = resolve;
  });
  const requests: URL[] = [];
  vi.stubGlobal("fetch", async (input: RequestInfo | URL) => {
    const url = new URL(input instanceof Request ? input.url : String(input));
    requests.push(url);
    const cursor = url.searchParams.get("cursor");
    const q = url.searchParams.get("q");
    if (cursor) await pending;
    return new Response(
      JSON.stringify({
        results: [
          {
            $type: "games.gamesgamesgamesgames.defs#gameSummaryView",
            uri: `at://did:plc:ho26ynsdw2ey7l56xx4owjzd/games.gamesgamesgamesgames.game/${q}`,
            name: cursor ? "Old page" : q,
          },
        ],
        ...(q === "old" && !cursor ? { cursor: "next" } : {}),
      }),
      { headers: { "Content-Type": "application/json" } },
    );
  });
  useStore.setState({ url: "https://node.example", pdsAgent: null });
  await act(async () =>
    root.render(
      <I18nextProvider i18n={i18n}>
        <ActivityPicker value={undefined} onChange={() => {}} />
      </I18nextProvider>,
    ),
  );
  const input = container.querySelector("input")!;
  async function change(value: string) {
    await act(async () => {
      Object.getOwnPropertyDescriptor(
        HTMLInputElement.prototype,
        "value",
      )!.set!.call(input, value);
      input.dispatchEvent(new Event("input", { bubbles: true }));
      await vi.advanceTimersByTimeAsync(300);
    });
  }
  await change("old");
  const list = container.querySelector('[data-testid="game-results"]')!;
  Object.defineProperties(list, {
    scrollTop: { value: 700 },
    scrollHeight: { value: 1000 },
    clientHeight: { value: 300 },
  });
  await act(async () => {
    list.dispatchEvent(new Event("scroll", { bubbles: true }));
    list.dispatchEvent(new Event("scroll", { bubbles: true }));
  });
  expect(requests).toHaveLength(2);
  await change("new");
  await act(async () => release());
  expect(container.textContent).toContain("new");
  expect(container.textContent).not.toContain("Old page");
});
