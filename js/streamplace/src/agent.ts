import { Agent } from "@atproto/api";
import { Client } from "@atproto/lex";

type SessionLike = {
  did?: string;
  fetchHandler: (url: string, init?: RequestInit) => Promise<Response>;
};

// The node's answer when the OAuth session a request carries is gone on its
// side: revoked, or not in its store any more. The client's own tokens can
// still look valid, so without noticing this the app stays "signed in"
// while every authenticated call fails.
const SESSION_REJECTED = /oauth session (revoked|not found)/i;
const REFRESH_GAVE_UP =
  /TokenRefreshError|TokenRevoked|TokenInvalid|invalid_grant/i;

export class StreamplaceAgent extends Agent {
  // The @atproto/lex client for Streamplace (place.stream.* / games.*) XRPC and
  // records. It rides on the same session/fetchHandler as the @atproto/api Agent,
  // which we keep for OAuth and Bluesky (app.bsky.*) reads.
  client: Client;

  /** Called once when the node rejects the session this agent holds: a 401
   *  saying the OAuth session is revoked or unknown, or the OAuth client
   *  giving up on a refresh. The holder should drop the session and ask for
   *  a sign-in; until it does, every authenticated call fails. */
  onSessionRejected?: (reason: string) => void;
  private rejected = false;

  constructor(options: ConstructorParameters<typeof Agent>[0]) {
    const holder: { agent?: StreamplaceAgent } = {};
    super(StreamplaceAgent.guarded(options, holder));
    holder.agent = this;

    this.client = new Client({
      did: this.did as `did:${string}:${string}` | undefined,
      fetchHandler: this.fetchHandler,
    });
  }

  // Wraps a session's fetch handler so both the api Agent and the lex Client
  // see the node's verdict on the session. A plain service URL is returned
  // as is: an anonymous agent has no session to lose.
  private static guarded(
    options: ConstructorParameters<typeof Agent>[0],
    holder: { agent?: StreamplaceAgent },
  ): ConstructorParameters<typeof Agent>[0] {
    if (
      typeof options !== "object" ||
      options === null ||
      !("fetchHandler" in options)
    ) {
      return options;
    }
    const session = options as SessionLike;
    const wrapped: SessionLike = {
      get did() {
        return session.did;
      },
      fetchHandler: async (url, init) => {
        let res: Response;
        try {
          res = await session.fetchHandler(url, init);
        } catch (err: any) {
          const text = `${err?.name ?? ""} ${err?.message ?? ""}`;
          if (REFRESH_GAVE_UP.test(text)) holder.agent?.reject(text.trim());
          throw err;
        }
        if (res.status === 401) {
          try {
            const body = await res.clone().text();
            if (SESSION_REJECTED.test(body)) {
              holder.agent?.reject(body.slice(0, 120));
            }
          } catch {
            // an unreadable body is not the answer we are looking for
          }
        }
        return res;
      },
    };
    return wrapped as ConstructorParameters<typeof Agent>[0];
  }

  private reject(reason: string) {
    if (this.rejected) return;
    this.rejected = true;
    this.onSessionRejected?.(reason);
  }
}
