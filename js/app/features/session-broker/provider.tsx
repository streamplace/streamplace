import { useBrandingAsset, useUrl } from "@streamplace/components";
import { useEffect, useRef } from "react";
import { Platform } from "react-native";
import { useStore } from "store";
import { BrokeredSession, BrokeredSessionData } from "streamplace";
import { SessionBrokerClient, startSessionBroker } from "./broker";

// How long the first frame waits for the broker before giving up on it.
const BROKER_TIMEOUT_MS = 4000;

/**
 * Inherits the login of a sibling app (branding key sessionBrokerOrigin)
 * on web: once the node's own OAuth restore has finished without a session,
 * a hidden iframe of the broker asks that app who is signed in and the
 * answer becomes a brokered session here. The broker's later pushes keep
 * the token fresh and end the session when the user signs out there.
 */
export function SessionBrokerProvider() {
  const origin = useBrandingAsset("sessionBrokerOrigin")?.data?.trim();
  const nodeUrl = useUrl();
  const authStatus = useStore((s) => s.authStatus);
  const sessionKind = useStore((s) => s.sessionKind);
  const setBrokeredSession = useStore((s) => s.setBrokeredSession);
  const setBrokerSettled = useStore((s) => s.setBrokerSettled);
  const clientRef = useRef<SessionBrokerClient | null>(null);
  const sessionRef = useRef<BrokeredSession | null>(null);

  const active =
    Platform.OS === "web" &&
    !!origin &&
    authStatus !== "start" &&
    sessionKind !== "oauth";

  useEffect(() => {
    if (!active) return;
    setBrokerSettled(false);
    const timeout = setTimeout(() => setBrokerSettled(true), BROKER_TIMEOUT_MS);
    const apply = (data: BrokeredSessionData | null, error?: string) => {
      setBrokerSettled(true);
      if (!data) {
        // "refresh-failed" means try again later, not logged out.
        if (error === "refresh-failed") return;
        sessionRef.current = null;
        setBrokeredSession(null);
        return;
      }
      const current = sessionRef.current;
      if (current && current.did === data.did) {
        current.update(data);
        return;
      }
      const session = new BrokeredSession(data, {
        nodeUrl: nodeUrl!,
        kind: "brokered",
        refresh: async () => {
          const client = clientRef.current;
          return client ? client.request() : null;
        },
      });
      sessionRef.current = session;
      setBrokeredSession(session);
    };
    const client = startSessionBroker(origin!, apply);
    clientRef.current = client;
    if (!client) setBrokerSettled(true);
    return () => {
      clearTimeout(timeout);
      client?.stop();
      clientRef.current = null;
      setBrokerSettled(true);
    };
  }, [active, origin, nodeUrl, setBrokeredSession, setBrokerSettled]);

  return null;
}
