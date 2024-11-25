import {
  BrowserOAuthClient,
  OAuthClientMetadata,
} from "@atproto/oauth-client-browser";

export type AquareumOAuthClient = Omit<
  BrowserOAuthClient,
  "keyset" | "serverFactory" | "jwks"
>;

export default async function createOAuthClient(
  aquareumUrl: string,
): Promise<AquareumOAuthClient> {
  if (!aquareumUrl) {
    throw new Error("aquareumUrl is required");
  }
  let meta: OAuthClientMetadata;
  if (
    aquareumUrl.startsWith("http://localhost") ||
    aquareumUrl.startsWith("http://127.0.0.1")
  ) {
    const u = new URL(document.location.href);

    // loopback client that doesn't require interaction with the server
    meta = {
      client_id: "http://localhost?scope=atproto%20transition:generic",
      redirect_uris: [`${u.protocol}//${u.host}`],
      scope: "atproto transition:generic",
      token_endpoint_auth_method: "none",
      // jwks_uri: "https://my-app.example/jwks.json",
      client_name: "Loopback client",
      response_types: ["code"],
      grant_types: ["authorization_code", "refresh_token"],
      application_type: "native",
      dpop_bound_access_tokens: true,
    };
  } else {
    const res = await fetch(`${aquareumUrl}/api/atproto-oauth`);
    meta = await res.json();
  }
  return new BrowserOAuthClient({
    handleResolver: "https://bsky.social", // backend instances should use a DNS based resolver
    responseMode: "query", // or "fragment" (frontend only) or "form_post" (backend only)

    // These must be the same metadata as the one exposed on the
    // "client_id" endpoint (except when using a loopback client)
    clientMetadata: meta,
  });
}
