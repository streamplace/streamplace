import { BrowserOAuthClient } from "@atproto/oauth-client-browser";

export default new BrowserOAuthClient({
  handleResolver: "https://bsky.social", // backend instances should use a DNS based resolver
  responseMode: "query", // or "fragment" (frontend only) or "form_post" (backend only)

  // These must be the same metadata as the one exposed on the
  // "client_id" endpoint (except when using a loopback client)
  clientMetadata: {
    client_id: "http://localhost?scope=atproto%20transition:generic",
    redirect_uris: ["http://127.0.0.1:38081"],
    scope: "atproto transition:generic",
    token_endpoint_auth_method: "none",
    // jwks_uri: "https://my-app.example/jwks.json",
    client_name: "Loopback client",
    response_types: ["code"],
    grant_types: ["authorization_code", "refresh_token"],
    application_type: "native",
    dpop_bound_access_tokens: true,
  },
});
