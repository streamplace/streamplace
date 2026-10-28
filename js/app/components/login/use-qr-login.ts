import { usePossiblyUnauthedPDSAgent, useUrl } from "@streamplace/components";
import { useEffect, useState } from "react";
import { useStore } from "store";
import { place } from "streamplace";

const POLL_MS = 2000;

// QR login, the browser's half: a pushed authorization request it never
// follows, registered with the node as a QR login that the Streamplace app
// (signed in to this node) scans and approves. Approval hands back the
// authorization response the redirect would have carried, and the OAuth
// client completes the login with it as usual. Returns what the QR code
// encodes: undefined while an attempt is being made, null if the node doesn't
// offer QR login. A denied or expired attempt is replaced.
export function useQrLogin(
  enabled: boolean,
  blueskyPermissions: boolean,
): string | null | undefined {
  const url = useUrl();
  const agent = usePossiblyUnauthedPDSAgent();
  const authorizeUrl = useStore((state) => state.authorizeUrl);
  const oauthCallback = useStore((state) => state.oauthCallback);
  const [attempt, setAttempt] = useState(0);
  const [qrUrl, setQrUrl] = useState<string | null | undefined>(undefined);

  useEffect(() => {
    if (!enabled || !agent || !url) return;
    let stopped = false;
    let timer: ReturnType<typeof setTimeout> | undefined;
    setQrUrl(undefined);

    const poll = async (id: string) => {
      try {
        const login = await agent.client.call(place.stream.login.getQRLogin, {
          id,
        });
        if (stopped) return;
        if (login.status === "approved" && login.authorization) {
          stopped = true;
          const { code, state, iss } = login.authorization;
          const query = new URLSearchParams({ code, state, iss });
          await oauthCallback(`${url}/login?${query}`);
          return;
        }
        if (login.status !== "pending") {
          setAttempt((n) => n + 1);
          return;
        }
      } catch (e) {
        // a dropped request isn't the end of the attempt; try again
        console.warn("qr login poll failed", e);
      }
      if (!stopped) timer = setTimeout(() => poll(id), POLL_MS);
    };

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
        setQrUrl(created.url);
        timer = setTimeout(() => poll(created.id), POLL_MS);
      } catch (e) {
        // QRLoginUnavailable on nodes without an app to approve with
        console.warn("qr login unavailable", e);
        if (!stopped) setQrUrl(null);
      }
    })();

    return () => {
      stopped = true;
      clearTimeout(timer);
    };
  }, [
    enabled,
    agent,
    url,
    blueskyPermissions,
    attempt,
    authorizeUrl,
    oauthCallback,
  ]);

  return qrUrl;
}
