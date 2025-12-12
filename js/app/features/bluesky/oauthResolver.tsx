import type { OAuthAuthorizationServerMetadata } from "@atproto/oauth-client";
import {
  OAuthResolver,
  ResolveOAuthOptions,
} from "@atproto/oauth-client/dist/oauth-resolver";

export class StreamplaceOAuthResolver extends OAuthResolver {
  private currentResourceServer: string | null = null;

  constructor(
    private streamplaceUrl: string,
    ...args: ConstructorParameters<typeof OAuthResolver>
  ) {
    super(...args);
  }

  async resolveFromService(
    input: string,
    options?: ResolveOAuthOptions,
  ): Promise<{
    metadata: OAuthAuthorizationServerMetadata;
  }> {
    // Input is the resource server URL (e.g., https://selfhosted.social)
    // Store it for use in login_hint
    this.currentResourceServer = input;

    // Always fetch metadata from our backend
    // The issuer will be our backend, not the resource server
    const metadata = await this.getResourceServerMetadata(
      this.streamplaceUrl,
      options,
    );

    return { metadata };
  }

  getCurrentResourceServer(): string | null {
    return this.currentResourceServer;
  }
}
