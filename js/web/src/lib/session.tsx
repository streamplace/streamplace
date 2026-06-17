// App-wide session context. Re-binds to the bluesky slice: the
// SessionProvider no longer owns the BrowserOAuthClient instance or
// runs `client.init()`. BlueskyProvider handles the OAuth lifecycle;
// this provider just exposes reactive bindings ({state, signIn,
// signOut, pdsAgent, did}) to consumers that want context semantics.
//
// `getClient` was dropped from the public API — nothing in the web
// uses it directly anymore, and callers who need the underlying
// client can read the slice's `client` field.
import { OAuthSession } from "@atproto/oauth-client-browser";
import {
  createContext,
  useCallback,
  useContext,
  useMemo,
  type ReactNode,
} from "react";
import { StreamplaceAgent } from "streamplace";
import { useStore } from "./store";

type SessionState =
  | { status: "loading" }
  | { status: "anonymous" }
  | { status: "authenticated"; session: OAuthSession };

type SessionContextValue = {
  state: SessionState;
  signIn: (handle: string, mode?: "popup" | "redirect") => Promise<void>;
  signOut: () => Promise<void>;
  pdsAgent: StreamplaceAgent | null;
  did: string | null;
};

const SessionContext = createContext<SessionContextValue | null>(null);

export function SessionProvider({ children }: { children: ReactNode }) {
  const authStatus = useStore((state) => state.authStatus);
  const oauthSession = useStore((state) => state.oauthSession);
  const pdsAgent = useStore((state) => state.pdsAgent);
  const login = useStore((state) => state.login);
  const logout = useStore((state) => state.logout);

  const state: SessionState = useMemo(() => {
    if (authStatus === "loggedIn" && oauthSession) {
      return { status: "authenticated", session: oauthSession };
    }
    if (authStatus === "loggedOut") {
      return { status: "anonymous" };
    }
    return { status: "loading" };
  }, [authStatus, oauthSession]);

  const signIn = useCallback(
    async (handle: string, mode: "popup" | "redirect" = "popup") => {
      await login(handle, mode);
    },
    [login],
  );

  const signOut = useCallback(async () => {
    try {
      await logout();
    } catch (e) {
      console.error("signOut error", e);
    }
  }, [logout]);

  const did = oauthSession?.did ?? null;

  const value = useMemo<SessionContextValue>(
    () => ({ state, signIn, signOut, pdsAgent, did }),
    [state, signIn, signOut, pdsAgent, did],
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
