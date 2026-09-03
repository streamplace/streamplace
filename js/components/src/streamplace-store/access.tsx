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
}

// What we assume when the node predates access control (the method doesn't
// exist there): everything open, no roles.
const OPEN_ACCESS_STATUS: AccessStatus = { roles: [], policy: {} };

// A node without place.stream.access.getStatus answers through its wildcard
// proxy or a 404, never with a policy. Anything else (network failure, a
// response the client can't parse) must not be mistaken for "open": on a
// private node that would render an app whose every request fails.
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
      const res = await agent.client.call(place.stream.access.getStatus);
      const policy: Record<string, string> = {};
      for (const entry of res.policy?.roles ?? []) {
        policy[entry.role] = entry.mode;
      }
      store.setState({
        accessStatus: {
          did: res.did,
          roles: [...(res.roles ?? [])],
          policy,
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

export const useAccessStatusLoaded = () =>
  useStreamplaceStore((s) => s.accessStatusLoaded);

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
    if (!s.accessStatusLoaded || !s.accessStatus) return false;
    const mode = s.accessStatus.policy.viewer;
    if (mode === undefined || mode === "open") return false;
    const roles = s.accessStatus.roles;
    return !roles.includes("viewer") && !roles.includes("admin");
  });
}

export const useAccessStatusError = () =>
  useStreamplaceStore((s) => s.accessStatusError);
