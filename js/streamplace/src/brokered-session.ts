// The shape @atproto/api's Agent accepts as a session manager, spelled out
// here because the package does not export the type from its entry point.
interface SessionManager {
  readonly did?: string;
  fetchHandler(url: string, init?: RequestInit): Promise<Response>;
}

/** A bearer session for the user's PDS: what a session broker hands over,
 *  or what com.atproto.server.createSession returns. */
export interface BearerSessionData {
  did: string;
  handle: string;
  /** The user's PDS (the token's audience). */
  service: string;
  accessJwt: string;
  /** Present when this app holds the session itself (credentials login). */
  refreshJwt?: string;
}

/** For compatibility with the first, broker-only shape. */
export type BrokeredSessionData = BearerSessionData;

export type BearerSessionKind = "brokered" | "credential";

export interface BearerSessionOptions {
  /** The node, for its own lexicons and API routes. */
  nodeUrl: string;
  kind?: BearerSessionKind;
  /** Broker-backed sessions: ask the broker for a fresh token. */
  refresh?: () => Promise<BearerSessionData | null>;
  /** Called with the new tokens after a refresh (to persist them). */
  onUpdate?: (data: BearerSessionData) => void;
  /** atproto-proxy value for app view reads (Bluesky's by default). */
  appViewProxy?: string;
}

/**
 * A session made of a bearer token for the user's PDS rather than an OAuth
 * session with the node, for a front end that inherits its login from a
 * sibling app (the session broker) or signs in against the PDS directly
 * (email/app password, or the PDS's QR login). The node cannot accept the
 * token, so this manager routes: the node's own methods (place.stream.*,
 * games.*, /api/*) go to the node anonymously, everything else goes to the
 * PDS with the bearer token. An expired token is refreshed once: through
 * the broker for brokered sessions, through refreshSession for ones this
 * app holds.
 */
export class BearerSession implements SessionManager {
  /** Lets callers that only see a SessionManager tell this kind apart. */
  readonly kind: BearerSessionKind;
  private data: BearerSessionData;
  private readonly nodeUrl: string;
  private readonly refreshViaBroker?: () => Promise<BearerSessionData | null>;
  private readonly onUpdate?: (data: BearerSessionData) => void;
  private refreshing: Promise<BearerSessionData | null> | null = null;
  readonly appViewProxy: string;

  constructor(data: BearerSessionData, options: BearerSessionOptions) {
    this.data = data;
    this.kind = options.kind ?? (options.refresh ? "brokered" : "credential");
    this.nodeUrl = options.nodeUrl.replace(/\/$/, "");
    this.refreshViaBroker = options.refresh;
    this.onUpdate = options.onUpdate;
    this.appViewProxy =
      options.appViewProxy ?? "did:web:api.bsky.app#bsky_appview";
  }

  get did(): string {
    return this.data.did;
  }

  get handle(): string {
    return this.data.handle;
  }

  get service(): string {
    return this.data.service;
  }

  /** A snapshot of the tokens, e.g. to persist. */
  snapshot(): BearerSessionData {
    return { ...this.data };
  }

  /** New tokens for the same account arrived (broker push, refresh). */
  update(data: BearerSessionData) {
    this.data = { ...this.data, ...data };
  }

  /** Ends a credential session at the PDS (best effort). */
  async signOut(): Promise<void> {
    if (!this.data.refreshJwt) return;
    try {
      await fetch(this.base() + "/xrpc/com.atproto.server.deleteSession", {
        method: "POST",
        headers: { Authorization: `Bearer ${this.data.refreshJwt}` },
      });
    } catch {
      // the local session is gone either way
    }
  }

  private base(): string {
    return this.data.service.replace(/\/$/, "");
  }

  /** app.bsky.* and chat.bsky.* are served by an app view behind the PDS. */
  private isAppViewBound(pathname: string): boolean {
    const nsid = pathname.slice("/xrpc/".length).split("?")[0];
    return nsid.startsWith("app.bsky.") || nsid.startsWith("chat.bsky.");
  }

  /** The node handles its own lexicons and API routes; the PDS the rest. */
  private isNodeBound(pathname: string): boolean {
    if (!pathname.startsWith("/xrpc/")) return true;
    const nsid = pathname.slice("/xrpc/".length).split("?")[0];
    return nsid.startsWith("place.stream.") || nsid.startsWith("games.");
  }

  async fetchHandler(pathname: string, init?: RequestInit): Promise<Response> {
    if (this.isNodeBound(pathname)) {
      return fetch(this.nodeUrl + pathname, init);
    }
    const send = () => {
      const headers = new Headers(init?.headers);
      headers.set("Authorization", `Bearer ${this.data.accessJwt}`);
      // App view reads go through the PDS's proxy; a PDS that has no default
      // app view (tranquil, for one) answers 501 without the header.
      if (this.isAppViewBound(pathname) && !headers.has("atproto-proxy")) {
        headers.set("atproto-proxy", this.appViewProxy);
      }
      return fetch(this.base() + pathname, { ...init, headers });
    };
    let res = await send();
    if (res.status === 400 || res.status === 401) {
      const body = await res
        .clone()
        .json()
        .catch(() => null);
      if (body?.error === "ExpiredToken") {
        const fresh = await this.refreshOnce();
        if (fresh) res = await send();
      }
    }
    return res;
  }

  private refreshOnce(): Promise<BearerSessionData | null> {
    if (!this.refreshing) {
      this.refreshing = this.doRefresh()
        .then((data) => {
          if (data) {
            this.update(data);
            this.onUpdate?.(this.snapshot());
          }
          return data;
        })
        .finally(() => {
          this.refreshing = null;
        });
    }
    return this.refreshing;
  }

  private async doRefresh(): Promise<BearerSessionData | null> {
    if (this.refreshViaBroker) return this.refreshViaBroker();
    if (!this.data.refreshJwt) return null;
    const res = await fetch(
      this.base() + "/xrpc/com.atproto.server.refreshSession",
      {
        method: "POST",
        headers: { Authorization: `Bearer ${this.data.refreshJwt}` },
      },
    );
    if (!res.ok) return null;
    const body = await res.json();
    if (!body?.accessJwt) return null;
    return {
      did: body.did ?? this.data.did,
      handle: body.handle ?? this.data.handle,
      service: this.data.service,
      accessJwt: body.accessJwt,
      refreshJwt: body.refreshJwt ?? this.data.refreshJwt,
    };
  }
}

/** The first name this class shipped under. */
export { BearerSession as BrokeredSession };
