import { useLoginOptions, useUrl } from "@streamplace/components";
import {
  createContext,
  ReactNode,
  useContext,
  useEffect,
  useMemo,
  useState,
} from "react";
import { Platform } from "react-native";
import { useStore } from "store";
import { place, StreamplaceAgent } from "streamplace";

const POLL_MS = 2000;
// replace an attempt this long before it expires, so the code on screen is
// always one the node will still take
const REFRESH_BEFORE_EXPIRY_MS = 30_000;
// whatever the clocks say, don't replace attempts more often than this
const MIN_REFRESH_MS = 60_000;

interface Attempt {
  id: string;
  url: string;
  blueskyPermissions: boolean;
}

interface QrLoginContextValue {
  // undefined while one is being made, null if the node doesn't offer QR login
  attempt: Attempt | null | undefined;
  setBlueskyPermissions: (blueskyPermissions: boolean) => void;
  setShown: (shown: boolean) => void;
}

const QrLoginContext = createContext<QrLoginContextValue | null>(null);

// QR login, the browser's half: a pushed authorization request it never
// follows, registered with the node as a QR login that the Streamplace app
// (signed in to this node) scans and approves. Approval hands back the
// authorization response the redirect would have carried, and the OAuth
// client completes the login with it as usual.
//
// While logged out on the web, an attempt is kept ready (and replaced before
// it expires), so the login form shows its code at once. It's only polled
// while a form shows it. A denied or expired attempt is replaced.
export function QrLoginProvider({ children }: { children: ReactNode }) {
  const url = useUrl();
  // QR logins are made and polled unauthenticated. An agent of its own, not
  // the store's, which is remade as the session loads.
  const agent = useMemo(() => (url ? new StreamplaceAgent(url) : null), [url]);
  const { methods } = useLoginOptions();
  // only once the OAuth client has loaded and found no session
  const loggedOut = useStore((state) => state.authStatus === "loggedOut");
  const authorizeUrl = useStore((state) => state.authorizeUrl);
  const oauthCallback = useStore((state) => state.oauthCallback);
  const [blueskyPermissions, setBlueskyPermissions] = useState(true);
  const [shown, setShown] = useState(false);
  const [generation, setGeneration] = useState(0);
  const [attempt, setAttempt] = useState<Attempt | null | undefined>();
  const enabled = Platform.OS === "web" && methods.includes("qr") && loggedOut;

  useEffect(() => {
    if (!enabled || !agent || !url) {
      setAttempt(undefined);
      return;
    }
    let stopped = false;
    let timer: ReturnType<typeof setTimeout> | undefined;
    // the code on screen stays up until its replacement is ready
    (async () => {
      try {
        const authorize = await authorizeUrl(url, { blueskyPermissions });
        const requestUri = authorize.searchParams.get("request_uri");
        if (!requestUri) throw new Error("authorize URL has no request_uri");
        const created = await agent.client.call(
          place.stream.login.createQRLogin,
          { requestUri },
        );
        if (stopped) return;
        setAttempt({ id: created.id, url: created.url, blueskyPermissions });
        const refreshIn =
          Date.parse(created.expiresAt) - Date.now() - REFRESH_BEFORE_EXPIRY_MS;
        timer = setTimeout(
          () => setGeneration((n) => n + 1),
          Math.max(refreshIn, MIN_REFRESH_MS),
        );
      } catch (e) {
        // QRLoginUnavailable on nodes without an app to approve with
        console.warn("qr login unavailable", e);
        if (!stopped) setAttempt(null);
      }
    })();
    return () => {
      stopped = true;
      clearTimeout(timer);
    };
  }, [enabled, agent, url, blueskyPermissions, generation, authorizeUrl]);

  useEffect(() => {
    if (!shown || !attempt || !agent || !url) return;
    let stopped = false;
    let timer: ReturnType<typeof setTimeout> | undefined;
    const poll = async () => {
      try {
        const login = await agent.client.call(place.stream.login.getQRLogin, {
          id: attempt.id,
        });
        if (stopped) return;
        if (login.status === "approved" && login.authorization) {
          const { code, state, iss } = login.authorization;
          const query = new URLSearchParams({ code, state, iss });
          await oauthCallback(`${url}/login?${query}`);
          // spent either way; signed in, this provider stops making them
          setGeneration((n) => n + 1);
          return;
        }
        if (login.status !== "pending") {
          setGeneration((n) => n + 1);
          return;
        }
      } catch (e) {
        // a dropped request isn't the end of the attempt; try again
        console.warn("qr login poll failed", e);
      }
      if (!stopped) timer = setTimeout(poll, POLL_MS);
    };
    timer = setTimeout(poll, POLL_MS);
    return () => {
      stopped = true;
      clearTimeout(timer);
    };
  }, [shown, attempt, agent, url, oauthCallback]);

  return (
    <QrLoginContext.Provider
      value={{ attempt, setBlueskyPermissions, setShown }}
    >
      {children}
    </QrLoginContext.Provider>
  );
}

// What the login form's QR code encodes: undefined while an attempt is being
// made, null if the node doesn't offer QR login.
export function useQrLogin(
  blueskyPermissions: boolean,
): string | null | undefined {
  const ctx = useContext(QrLoginContext);
  if (!ctx) throw new Error("useQrLogin needs a QrLoginProvider");
  const { attempt, setBlueskyPermissions, setShown } = ctx;

  useEffect(() => {
    setShown(true);
    return () => setShown(false);
  }, [setShown]);

  useEffect(() => {
    setBlueskyPermissions(blueskyPermissions);
  }, [blueskyPermissions, setBlueskyPermissions]);

  // one made for other permissions is about to be replaced
  if (attempt && attempt.blueskyPermissions !== blueskyPermissions) {
    return undefined;
  }
  return attempt === null ? null : attempt?.url;
}
