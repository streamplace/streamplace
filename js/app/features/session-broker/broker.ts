import { Platform } from "react-native";
import type { BrokeredSessionData } from "streamplace";

// The sibling app's /session-broker route, loaded in a hidden iframe. It
// answers a request with the current login and pushes again whenever that
// changes; the token it hands over is a bearer token for the user's PDS,
// and the broker keeps it fresh (we never hold the refresh token). Protocol
// from the broker's README: request {type: "wsocial:session:request"},
// response {type: "wsocial:session", session, error?}.
const REQUEST = "wsocial:session:request";
const RESPONSE = "wsocial:session";

export interface SessionBrokerClient {
  /** Ask the broker for the current session; resolves with the next answer. */
  request(): Promise<BrokeredSessionData | null>;
  stop(): void;
}

export function startSessionBroker(
  origin: string,
  onSession: (session: BrokeredSessionData | null, error?: string) => void,
): SessionBrokerClient | null {
  if (Platform.OS !== "web" || typeof document === "undefined") return null;
  const brokerOrigin = origin.replace(/\/$/, "");
  const frame = document.createElement("iframe");
  frame.src = `${brokerOrigin}/session-broker`;
  frame.style.display = "none";
  frame.setAttribute("aria-hidden", "true");
  let loaded = false;
  let answered = false;
  let waiters: ((s: BrokeredSessionData | null) => void)[] = [];
  // The broker is a full app that boots inside the iframe; a request sent
  // on the frame's load event can land before its listener exists, so keep
  // asking until the first answer.
  let retry: ReturnType<typeof setInterval> | null = null;
  const RETRY_MS = 750;
  const RETRY_MAX = 40;

  const onMessage = (event: MessageEvent) => {
    if (event.origin !== brokerOrigin) return;
    const data = event.data;
    if (!data || typeof data !== "object" || data.type !== RESPONSE) return;
    answered = true;
    if (retry) {
      clearInterval(retry);
      retry = null;
    }
    const session = normalize(data.session);
    const error = typeof data.error === "string" ? data.error : undefined;
    onSession(session, error);
    const pending = waiters;
    waiters = [];
    for (const w of pending) w(session);
  };
  const post = () => {
    frame.contentWindow?.postMessage({ type: REQUEST }, brokerOrigin);
  };
  window.addEventListener("message", onMessage);
  frame.addEventListener("load", () => {
    loaded = true;
    post();
    let tries = 0;
    retry = setInterval(() => {
      if (answered || ++tries > RETRY_MAX) {
        if (retry) clearInterval(retry);
        retry = null;
        return;
      }
      post();
    }, RETRY_MS);
  });
  document.body.appendChild(frame);

  return {
    request: () =>
      new Promise((resolve) => {
        waiters.push(resolve);
        if (loaded) post();
      }),
    stop: () => {
      if (retry) clearInterval(retry);
      window.removeEventListener("message", onMessage);
      frame.remove();
    },
  };
}

function normalize(raw: any): BrokeredSessionData | null {
  if (!raw || typeof raw !== "object") return null;
  const { did, handle, service, accessJwt } = raw;
  if (
    typeof did !== "string" ||
    typeof handle !== "string" ||
    typeof service !== "string" ||
    typeof accessJwt !== "string"
  ) {
    return null;
  }
  return { did, handle, service, accessJwt };
}
