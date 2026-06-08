// Duck-typed OAuthResolver — avoids pnpm hoisting issues with @atproto/oauth-client base class.
// Overrides resolveFromService to route through the Streamplace backend.

export interface OAuthResolverShape {
  resolveFromService(
    input: string,
    options?: Record<string, unknown>,
  ): Promise<{ metadata: Record<string, unknown> }>;
}

export class StreamplaceOAuthResolver {
  private currentResourceServer: string | null = null;

  constructor(private streamplaceUrl: string) {}

  async resolveFromService(
    input: string,
    _options?: Record<string, unknown>,
  ): Promise<{ metadata: Record<string, unknown> }> {
    this.currentResourceServer = input;

    const metadataUrl = `${this.streamplaceUrl}/.well-known/oauth-authorization-server`;
    const res = await fetch(metadataUrl, {
      headers: { accept: "application/json" },
    });
    if (!res.ok) {
      throw new Error(
        `Failed to fetch authorization-server metadata from ${metadataUrl}: ${res.status}`,
      );
    }
    const metadata = await res.json();
    return { metadata };
  }

  getCurrentResourceServer(): string | null {
    return this.currentResourceServer;
  }
}
