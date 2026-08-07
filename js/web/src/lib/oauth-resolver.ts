// Duck-typed OAuthResolver — avoids pnpm hoisting issues with @atproto/oauth-client base class.
// Overrides resolveFromService to route through the Streamplace backend.
//
// Why not extend the real `OAuthResolver`? @atproto/oauth-client 0.7.x
// restricts its `exports` to the package root, so deep-importing
// `dist/oauth-resolver` (the only place `OAuthResolver` is exported) is
// blocked. The main entry doesn't re-export it. Adding a direct
// `@atproto/oauth-client` dep just to extend one class felt heavy, so we
// mirror the public surface here.
//
// Surface we need to satisfy (per @atproto/oauth-client 0.7.x):
//   - resolve(input)              — high-level dispatcher; called by OAuthClient.authorize
//   - resolveFromIdentity(input)  — called by OAuthServerAgent.verifyIssuer
//   - resolveFromService(input)   — the original override target
//   - getCurrentResourceServer()  — used by oauth.ts to inject login_hint on PAR
//
// All three resolve* methods return streamplace's OAuth server metadata
// regardless of input. The PAR / authorize flow injects `login_hint: input`
// (or the resolved PDS URL) so the streamplace backend can identify the
// user. The fetch hijack in oauth.ts rewrites the PDS service endpoint of
// the DID document response to streamplace, so a future base-class-style
// resolveFromIdentity would also land on streamplace's metadata.

export interface OAuthResolverShape {
  resolve(
    input: string,
    options?: Record<string, unknown>,
  ): Promise<{ metadata: Record<string, unknown> }>;
  resolveFromService(
    input: string,
    options?: Record<string, unknown>,
  ): Promise<{ metadata: Record<string, unknown> }>;
  resolveFromIdentity(
    input: string,
    options?: Record<string, unknown>,
  ): Promise<{ metadata: Record<string, unknown>; pds: URL }>;
  getCurrentResourceServer(): string | null;
}

export class StreamplaceOAuthResolver {
  private currentResourceServer: string | null = null;

  constructor(private streamplaceUrl: string) {}

  // High-level dispatcher. The base class branches on /^https?:\/\//
  // (URLs → resolveFromService, handles → resolveFromIdentity). Both
  // paths land on streamplace's metadata for us, so we just call
  // resolveFromService unconditionally.
  async resolve(
    input: string,
    options?: Record<string, unknown>,
  ): Promise<{ metadata: Record<string, unknown> }> {
    return this.resolveFromService(input, options);
  }

  async resolveFromIdentity(
    _input: string,
    _options?: Record<string, unknown>,
  ): Promise<{ metadata: Record<string, unknown>; pds: URL }> {
    // Returns streamplace's metadata plus a pds URL pointing at the
    // streamplace backend. The verifyIssuer check in
    // OAuthServerAgent uses resolved.pds.href; pointing it at
    // streamplace matches the client-side issuer, so the check
    // passes. Real identity resolution happens on the streamplace
    // backend during the PAR / callback roundtrip.
    const { metadata } = await this.resolveFromService(_input, _options);
    return { metadata, pds: new URL(this.streamplaceUrl) };
  }

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
