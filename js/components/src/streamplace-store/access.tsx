import { useCallback, useEffect } from "react";
import { place } from "streamplace";
import {
  getStreamplaceStoreFromContext,
  useStreamplaceStore,
} from "./streamplace-store";
import { usePossiblyUnauthedPDSAgent } from "./xrpc";

// Role-based access control state for the current node, as reported by
// place.stream.access.getStatus. `roles` are the caller's roles (empty when
// logged out), `policy` maps role -> mode ("open" | "allowlist" | "off").
export interface AccessStatus {
  did?: string;
  roles: string[];
  policy: Record<string, string>;
  /** Chat is restricted to users the node's trusted verifiers vouch for. */
  chatVerifiedOnly?: boolean;
  /** The caller is one of them. */
  chatVerified?: boolean;
  /** Whether the account lives on the network's own PDS; undefined when
   *  the node has no network PDS configured. */
  networkMember?: boolean;
}

// What we assume when the node predates access control (the method doesn't
// exist there): everything open, no roles.
const OPEN_ACCESS_STATUS: AccessStatus = { roles: [], policy: {} };

// A node without place.stream.access.getStatus answers through its wildcard
// proxy or a 404, never with a policy. Anything else (network failure, a
// response the client can't parse) must not be mistaken for "open": on a
// private node that would render an app whose every request fails.
// A request the node refused for want of a valid token: a 401, or the OAuth
// client giving up on a refresh (revoked or long-expired session).
function isAuthFailure(err: any): boolean {
  const status = err?.status ?? err?.statusCode;
  if (status === 401) return true;
  const text = `${err?.name ?? ""} ${err?.error ?? ""} ${err?.message ?? ""}`;
  return /TokenRefreshError|TokenRevoked|TokenInvalid|invalid_token|invalid_grant|InvalidToken|ExpiredToken|AuthenticationRequired|AuthMissing|revoked/i.test(
    text,
  );
}

function isMethodMissing(err: any): boolean {
  const status = err?.status ?? err?.statusCode;
  if (status === 404 || status === 501) return true;
  const text = `${err?.error ?? ""} ${err?.message ?? ""}`;
  return /MethodNotImplemented|not implemented|not found|unknown method/i.test(
    text,
  );
}

// Fetches place.stream.access.getStatus. Uses the possibly-authed agent so a
// logged-in caller gets their own DID + roles back; it is the one method a
// private node always answers, so a failure here means the node predates RBAC
// (or is unreachable) and we fall back to treating it as open.
export function useFetchAccessStatus() {
  const agent = usePossiblyUnauthedPDSAgent();
  const store = getStreamplaceStoreFromContext();

  return useCallback(async () => {
    try {
      if (!agent) {
        throw new Error("Streamplace agent not available");
      }
      // Name the account whenever there is one: a session the node can't
      // attribute (inherited from the network's app, made from a password,
      // or an OAuth session whose proof the node's proxy setup doesn't
      // honour) is anonymous to it, and the answer must still carry the
      // user's chat verification. The node ignores subject when it can
      // attribute the caller itself.
      const session = store.getState().oauthSession as
        | { did?: string; kind?: string }
        | null
        | undefined;
      const res = await agent.client.call(
        place.stream.access.getStatus,
        session?.did
          ? { subject: session.did as `did:${string}:${string}` }
          : {},
      );
      // An OAuth session the node didn't attribute is one it no longer
      // knows (revoked, or expired past refresh): the node answers such a
      // request as anonymous rather than with a 401. Left alone, the
      // answer's DID never matches the session's and the shell waits
      // forever for a status "for this caller". Hand the session to the
      // app to drop; the status is fetched again anonymously once it's
      // gone. Bearer sessions (brokered, password) are never attributed
      // and sign themselves out when they die (BearerSession.onExpired).
      const bearer =
        session?.kind === "brokered" || session?.kind === "credential";
      if (session?.did && !bearer && !res.did) {
        const { onSessionInvalid } = store.getState();
        if (onSessionInvalid) {
          console.warn(
            "Access status: the node no longer recognises this session, dropping it",
          );
          onSessionInvalid();
          return;
        }
      }
      const policy: Record<string, string> = {};
      for (const entry of res.policy?.roles ?? []) {
        policy[entry.role] = entry.mode;
      }
      store.setState({
        accessStatus: {
          did: res.did,
          roles: [...(res.roles ?? [])],
          policy,
          chatVerifiedOnly: res.chatVerifiedOnly ?? false,
          chatVerified: res.chatVerified ?? false,
          networkMember: res.networkMember ?? undefined,
        },
        accessStatusLoaded: true,
        accessStatusError: null,
      });
    } catch (err: any) {
      console.error("Failed to fetch access status:", err);
      if (isMethodMissing(err)) {
        store.setState({
          accessStatus: OPEN_ACCESS_STATUS,
          accessStatusLoaded: true,
          accessStatusError: null,
        });
        return;
      }
      // The node won't take this session's token any more (revoked, or
      // expired beyond refresh). That is a signed-out viewer, not an
      // unreachable node: hand the session back to the app to drop, and the
      // status is fetched again anonymously once it's gone. Bearer sessions
      // sign themselves out (BearerSession.onExpired) so only OAuth ones
      // get here.
      const { oauthSession, onSessionInvalid } = store.getState();
      if (oauthSession && onSessionInvalid && isAuthFailure(err)) {
        console.warn(
          "Access status: session rejected by the node, dropping it",
        );
        onSessionInvalid();
        return;
      }
      store.setState({
        accessStatusLoaded: false,
        accessStatusError: err?.message ?? String(err),
      });
    }
  }, [agent, store]);
}

// Fetches access status as soon as the app boots and again whenever the
// logged-in DID changes (session restored, login, logout), so the wall and
// admin gating always reflect the caller.
//
// The first fetch is deliberately not held back until the OAuth client has
// restored the session: on a private node the anonymous answer is what tells
// the shell to show a wall instead of the app, and restoring can take long
// enough (it fetches client metadata) that waiting would let the shell's
// timeout render the app to a locked-out visitor. A logged-in caller gets a
// second answer the moment the session resolves.
export function useAccessStatusAutoFetch() {
  const fetchAccessStatus = useFetchAccessStatus();
  const store = getStreamplaceStoreFromContext();
  const oauthSession = useStreamplaceStore((s) => s.oauthSession);
  const did = oauthSession?.did;
  // While a signed-in viewer is locked out of a verified-only chat, ask
  // again every minute: the node re-checks verification on a schedule, so
  // a viewer who just got verified unlocks without reloading.
  const lockedOut = useChatLockedOut();
  useEffect(() => {
    if (!lockedOut || !did) return;
    const handle = setInterval(() => void fetchAccessStatus(), 60_000);
    return () => clearInterval(handle);
  }, [lockedOut, did, fetchAccessStatus]);

  useEffect(() => {
    const { accessStatus, accessStatusLoaded } = store.getState();
    // The caller changed since the last answer: drop the stale one so the
    // shell holds a blank frame instead of a wrong wall while we refetch.
    if (accessStatusLoaded && accessStatus?.did !== did) {
      store.setState({ accessStatusLoaded: false });
    }
    fetchAccessStatus();
  }, [did, fetchAccessStatus, store]);
}

export const useAccessStatus = () => useStreamplaceStore((s) => s.accessStatus);

// An answer only counts once it belongs to the current caller. When the
// session restores (or the user signs in or out) the store re-renders before
// the effect that refetches runs, so for one frame the old anonymous answer
// would otherwise pair with a signed-in session and paint the wall.
function statusIsCurrent(s: {
  accessStatusLoaded: boolean;
  accessStatus: AccessStatus | null;
  oauthSession: { did?: string; kind?: string } | null | undefined;
}): boolean {
  if (!s.accessStatusLoaded || !s.accessStatus) return false;
  if (s.oauthSession === undefined) return false;
  // A brokered session (a sibling app's bearer token) is anonymous to the
  // node, so the node's answer for it carries no DID.
  const kind = s.oauthSession?.kind;
  const expected =
    kind === "brokered" || kind === "credential"
      ? undefined
      : (s.oauthSession?.did ?? undefined);
  return (s.accessStatus.did ?? undefined) === expected;
}

export const useAccessStatusLoaded = () =>
  useStreamplaceStore((s) => statusIsCurrent(s));

// Whether the caller holds `role`. Admin implies every other role.
export function useHasRole(role: string): boolean {
  return useStreamplaceStore((s) => {
    const roles = s.accessStatus?.roles;
    if (!roles) return false;
    return roles.includes(role) || roles.includes("admin");
  });
}

export const useIsAdmin = () => useHasRole("admin");

// True when the node's viewer role is gated (allowlist/off) and the caller
// doesn't hold it, i.e. the frontend should show the access wall instead of
// the app. False until status has loaded, and for nodes without a policy.
export function useViewerLockedOut(): boolean {
  return useStreamplaceStore((s) => {
    if (!statusIsCurrent(s) || !s.accessStatus) return false;
    const mode = s.accessStatus.policy.viewer;
    if (mode === undefined || mode === "open") return false;
    const roles = s.accessStatus.roles;
    return !roles.includes("viewer") && !roles.includes("admin");
  });
}

// True when the node gates who may stream (the streamer role is allowlist
// or off) and the caller doesn't hold it: the go-live page should say who
// can, instead of offering a stream that the node will refuse. False until
// status has loaded, and for nodes without a policy.
export function useStreamerLockedOut(): boolean {
  return useStreamplaceStore((s) => {
    if (!statusIsCurrent(s) || !s.accessStatus) return false;
    const mode = s.accessStatus.policy.streamer;
    if (mode === undefined || mode === "open") return false;
    const roles = s.accessStatus.roles;
    return !roles.includes("streamer") && !roles.includes("admin");
  });
}

export const useAccessStatusError = () =>
  useStreamplaceStore((s) => s.accessStatusError);

/**
 * Whether the signed-in caller is locked out of chat: the node restricts
 * chat to verified users and the caller isn't one. False while logged out
 * (the composer already asks for a login then).
 */
export function useChatLockedOut(): boolean {
  return useStreamplaceStore(
    (s) =>
      !!s.accessStatus?.chatVerifiedOnly &&
      !!s.oauthSession?.did &&
      !s.accessStatus?.chatVerified,
  );
}
