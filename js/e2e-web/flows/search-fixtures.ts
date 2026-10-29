import type { Page } from "@playwright/test";

// Narrow boundary fixtures for the user-search flows. The node's own search
// and live status stay real in the tests that don't need many rows or
// pictures; these stand in for the two things the harness can't provide:
// more than one searchable account, and Bluesky profile pictures (the app
// looks them up on public.api.bsky.app, which knows nothing of harness DIDs).
export type Actor = { did: string; handle: string };

const SEARCH_URL = "**/xrpc/place.stream.live.searchActorsTypeahead*";
const PROFILES_URL =
  "https://public.api.bsky.app/xrpc/app.bsky.actor.getProfiles*";
const AVATAR_ORIGIN = "https://search-avatars.test";

const CORS = { "access-control-allow-origin": "*" };

// Accounts that exist nowhere else, so none of them is ever live.
export function fixtureActors(count: number): Actor[] {
  return Array.from({ length: count }, (_, i) => ({
    did: `did:plc:searchfixture${String.fromCharCode(97 + i).padStart(11, "a")}`,
    handle: `search-fixture-${i}.example.test`,
  }));
}

export const avatarUrl = (did: string) =>
  `${AVATAR_ORIGIN}/${encodeURIComponent(did)}.svg`;

// Answers every search with `actors`, whatever was typed.
export async function stubSearch(page: Page, actors: Actor[]) {
  await page.route(SEARCH_URL, (route) =>
    route.fulfill({
      status: 200,
      contentType: "application/json",
      headers: CORS,
      body: JSON.stringify({ actors }),
    }),
  );
}

// Give these accounts profile pictures that really load; other accounts have
// no profile, exercising the normal initial fallback.
export async function stubProfiles(page: Page, withAvatar: Actor[]) {
  await page.route(PROFILES_URL, (route) => {
    const request = route.request();
    if (request.method() === "OPTIONS") {
      return route.fulfill({
        status: 204,
        headers: {
          ...CORS,
          "access-control-allow-headers": "*",
          "access-control-allow-methods": "GET",
        },
      });
    }
    const asked = new URL(request.url()).searchParams.getAll("actors");
    return route.fulfill({
      status: 200,
      contentType: "application/json",
      headers: CORS,
      body: JSON.stringify({
        profiles: withAvatar
          .filter((a) => asked.includes(a.did))
          .map((a) => ({ ...a, avatar: avatarUrl(a.did) })),
      }),
    });
  });

  // A distinct colour per account, so a picture can't pass for another's.
  await page.route(`${AVATAR_ORIGIN}/**`, (route) => {
    const did = decodeURIComponent(
      new URL(route.request().url()).pathname.slice(1).replace(/\.svg$/, ""),
    );
    const hue = [...did].reduce((sum, c) => sum + c.charCodeAt(0), 0) % 360;
    return route.fulfill({
      status: 200,
      contentType: "image/svg+xml",
      headers: CORS,
      body: `<svg xmlns="http://www.w3.org/2000/svg" width="64" height="64" viewBox="0 0 64 64"><rect width="64" height="64" fill="hsl(${hue} 70% 50%)"/></svg>`,
    });
  });
}
