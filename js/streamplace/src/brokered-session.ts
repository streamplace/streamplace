import type { SessionManager } from "@atproto/api/dist/session-manager";

/** What a session broker hands over: a bearer token for the user's PDS. */
export interface BrokeredSessionData {
  did: string;
  handle: string;
  /** The user's PDS (the token's audience). */
  service: string;
  accessJwt: string;
}

/**
 * A session made of a bearer token for the user's PDS rather than an OAuth
 * session with the node, for a front end that inherits its login from a
 * sibling app (see the session broker). The node cannot accept the token,
 * so this manager routes: the node's own methods (place.stream.*, games.*,
 * /api/*) go to the node anonymously, everything else goes to the PDS with
 * the bearer token. On an expired token it asks the broker for a fresh one
 * and retries once.
 */
export class BrokeredSession implements SessionManager {
  private data: BrokeredSessionData;
  private readonly nodeUrl: string;
  private readonly refresh?: () => Promise<BrokeredSessionData | null>;
  private refreshing: Promise<BrokeredSessionData | null> | null = null;

  constructor(
    data: BrokeredSessionData,
    nodeUrl: string,
    refresh?: () => Promise<BrokeredSessionData | null>,
  ) {
    this.data = data;
    this.nodeUrl = nodeUrl.replace(/\/$/, "");
    this.refresh = refresh;
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

  /** The broker pushed a new token for the same account. */
  update(data: BrokeredSessionData) {
    this.data = data;
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
    const base = this.data.service.replace(/\/$/, "");
    const send = () => {
      const headers = new Headers(init?.headers);
      headers.set("Authorization", `Bearer ${this.data.accessJwt}`);
      return fetch(base + pathname, { ...init, headers });
    };
    let res = await send();
    if (res.status === 400 && this.refresh) {
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

  private refreshOnce(): Promise<BrokeredSessionData | null> {
    if (!this.refresh) return Promise.resolve(null);
    if (!this.refreshing) {
      this.refreshing = this.refresh()
        .then((data) => {
          if (data) this.update(data);
          return data;
        })
        .finally(() => {
          this.refreshing = null;
        });
    }
    return this.refreshing;
  }
}
