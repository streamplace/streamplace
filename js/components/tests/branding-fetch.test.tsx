import { afterEach, expect, it } from "vitest";
import { fetchBrandingOnce } from "../src/streamplace-store/branding";
import { makeStreamplaceStore } from "../src/streamplace-store/streamplace-store";

const URL = "https://node.example";

function brandMeta(key: string, data: string) {
  const meta = document.createElement("meta");
  meta.name = `internal-brand:${key}`;
  meta.content = JSON.stringify({ key, mimeType: "text/plain", data });
  document.head.appendChild(meta);
}

afterEach(() => {
  document.head
    .querySelectorAll('meta[name^="internal-brand:"]')
    .forEach((meta) => meta.remove());
  localStorage.clear();
});

// The page's branding is a snapshot from when it was served. The branding
// admin edits a key, refetches, then something re-runs the background fetch:
// that must not put the page's old value back over the admin's edit.
it("a background fetch keeps branding fetched since the page loaded", async () => {
  brandMeta("loginMethods", '["qr"]');
  const store = makeStreamplaceStore({ url: URL });
  const fetched = {
    loginMethods: {
      key: "loginMethods",
      mimeType: "text/plain",
      data: '["apple","qr"]',
    },
  };
  store.setState({ branding: fetched });

  await fetchBrandingOnce({
    force: false,
    broadcasterDID: "did:web:node.example",
    streamplaceAgent: null,
    url: URL,
    store,
  });

  expect(store.getState().branding).toBe(fetched);
});

it("a background fetch fills an empty store from the page", async () => {
  brandMeta("loginMethods", '["qr"]');
  const store = makeStreamplaceStore({ url: URL });

  await fetchBrandingOnce({
    force: false,
    broadcasterDID: "did:web:node.example",
    streamplaceAgent: null,
    url: URL,
    store,
  });

  expect(store.getState().branding?.loginMethods?.data).toBe('["qr"]');
});

// Native, and web behind the dev proxy, have no page snapshot and start from
// the copy saved last launch. An operator's change has to show on the next
// launch, not once that copy is an hour old.
it("a background fetch refreshes over the saved copy", async () => {
  localStorage.setItem(
    "branding:did:web:node.example",
    JSON.stringify({
      timestamp: Date.now(),
      data: { loginMethods: { key: "loginMethods", data: "[]" } },
    }),
  );
  const store = makeStreamplaceStore({ url: URL });
  const agent = {
    client: {
      call: async () => ({
        assets: [
          { key: "loginMethods", mimeType: "text/plain", data: '["qr"]' },
        ],
      }),
    },
  };

  await fetchBrandingOnce({
    force: false,
    broadcasterDID: "did:web:node.example",
    streamplaceAgent: agent as any,
    url: URL,
    store,
  });

  expect(store.getState().branding?.loginMethods?.data).toBe('["qr"]');
});
