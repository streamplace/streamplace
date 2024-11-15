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

  // keyset: [
  //   // For backend clients only, a list of private keys to use for signing
  //   // credentials. These keys MUST correspond to the public keys exposed on the
  //   // "jwks_uri" of the client metadata. Note that the jwks JSON corresponding
  //   // to the following keys can be obtained using the `client.jwks` getter.
  //   await JoseKey.fromImportable(process.env.PRIVATE_KEY_1),
  //   await JoseKey.fromImportable(process.env.PRIVATE_KEY_2),
  //   await JoseKey.fromImportable(process.env.PRIVATE_KEY_3),
  // ],
});
