import type { OAuthResolver } from "@atproto/oauth-client/dist/oauth-resolver";

// The SDK's server factory captures this object during construction. Configure
// its methods in place so authorization, callbacks and refresh share routing.
export function routeOAuthThroughStreamplace(
  resolver: OAuthResolver,
  streamplaceUrl: string,
): () => string | null {
  let loginHint: string | null = null;
  resolver.resolveFromService = async (input, options) => {
    loginHint = input;
    const metadata = await resolver.getResourceServerMetadata(
      streamplaceUrl,
      options,
    );
    return { metadata };
  };
  resolver.resolveFromIdentity = async (input, options) => {
    const identityInfo = await resolver.resolveIdentity(input, options);
    const { metadata } = await resolver.resolveFromService(input, options);
    return { identityInfo, metadata, pds: new URL(streamplaceUrl) };
  };
  return () => loginHint;
}
