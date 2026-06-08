// App-wide session context wrapping the ATProto BrowserOAuthClient.
import {
  BrowserOAuthClient,
  type OAuthSession,
} from "@atproto/oauth-client-browser";
import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useState,
  type ReactNode,
} from "react";
import { StreamplaceAgent } from "streamplace";
import createOAuthClient from "./oauth";
import { getStreamplaceUrl } from "./streamplace-url";

type SessionState =
  | { status: "loading" }
  | { status: "anonymous" }
  | { status: "authenticated"; session: OAuthSession };

type SessionContextValue = {
  state: SessionState;
  signIn: (handle: string) => Promise<void>;
  signOut: () => Promise<void>;
  getClient: () => Promise<BrowserOAuthClient>;
  /**
   * Authenticated StreamplaceAgent. Null when signed out or still
   * restoring the session. The agent is memoized off the OAuthSession
   * — it's safe to put in dependency arrays.
   */
  pdsAgent: StreamplaceAgent | null;
  /** Convenience: the authenticated user's DID, or null. */
  did: string | null;
};

const SessionContext = createContext<SessionContextValue | null>(null);

export function SessionProvider({ children }: { children: ReactNode }) {
  const [state, setState] = useState<SessionState>({ status: "loading" });
  const [client, setClient] = useState<BrowserOAuthClient | null>(null);

  const getClient = useCallback(async () => {
    if (client) return client;
    const c = await createOAuthClient(getStreamplaceUrl());
    setClient(c);
    return c;
  }, [client]);

  useEffect(() => {
    let cancelled = false;
    (async () => {
      try {
        const c = await getClient();
        if (cancelled) return;
        const result = await c.init();
        if (cancelled) return;
        if (result) {
          setState({ status: "authenticated", session: result.session });
        } else {
          setState({ status: "anonymous" });
        }
      } catch {
        if (cancelled) return;
        setState({ status: "anonymous" });
      }
    })();
    return () => {
      cancelled = true;
    };
  }, [getClient]);

  const signIn = useCallback(
    async (handle: string) => {
      const c = await getClient();
      await c.signIn(handle);
    },
    [getClient],
  );

  const signOut = useCallback(async () => {
    if (state.status !== "authenticated") return;
    await state.session.signOut();
    setState({ status: "anonymous" });
  }, [state]);

  const pdsAgent = useMemo<StreamplaceAgent | null>(() => {
    if (state.status !== "authenticated") return null;
    const session = state.session;
    return new StreamplaceAgent({
      did: session.did,
      fetchHandler: (pathname, init) => session.fetchHandler(pathname, init),
    });
  }, [state]);

  const did = state.status === "authenticated" ? state.session.did : null;

  const value = useMemo<SessionContextValue>(
    () => ({ state, signIn, signOut, getClient, pdsAgent, did }),
    [state, signIn, signOut, getClient, pdsAgent, did],
  );

  return (
    <SessionContext.Provider value={value}>{children}</SessionContext.Provider>
  );
}

export function useSession(): SessionContextValue {
  const ctx = useContext(SessionContext);
  if (!ctx) {
    throw new Error("useSession must be used within a <SessionProvider>");
  }
  return ctx;
}
