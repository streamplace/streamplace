// OAuth scope handling for optional Bluesky permissions. The scope values
// mirror pkg/atproto/lexicon_permission_sets.go on the Go side.

export const SCOPE_BSKY_POST_CREATE =
  "repo?collection=app.bsky.feed.post&action=create";
export const SCOPE_BSKY_ACTOR_STATUS = "repo?collection=app.bsky.actor.status";

// Scopes that let Streamplace write to the user's Bluesky account. The
// read-only rpc:app.bsky.* scopes are intentionally not included here.
const BSKY_REPO_SCOPE_PREFIX = "repo?collection=app.bsky.";

// Strip the scopes that grant write access to the user's Bluesky account,
// for logins where the user declined Bluesky permissions.
export function withoutBlueskyScopes(scope: string): string {
  return scope
    .split(" ")
    .filter((s) => !s.startsWith(BSKY_REPO_SCOPE_PREFIX))
    .join(" ");
}

// Whether the current session's granted scope includes `wanted`. A null
// sessionScope means we couldn't determine it (older server or the
// introspection call failed); those sessions predate declinable permissions
// and were granted everything, so treat unknown as granted.
export function scopeGrants(
  sessionScope: string | null,
  wanted: string,
): boolean {
  if (!sessionScope) {
    return true;
  }
  return sessionScope.split(" ").includes(wanted);
}
