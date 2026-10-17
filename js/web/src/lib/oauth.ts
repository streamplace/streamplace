// Builds an ATProto BrowserOAuthClient with downstream-OAuth support.
// Wraps fetch() to handle loopback rewrites and DID-doc endpoint rewriting.

import {
  BrowserOAuthClient,
  type ClientMetadata,
  clientMetadataSchema,
} from "@atproto/oauth-client-browser";
import { StreamplaceOAuthResolver } from "./oauth-resolver";
import { getCallbackUrl } from "./streamplace-callback";

export default async function createOAuthClient(
  streamplaceUrl: string,
): Promise<BrowserOAuthClient> {
  if (!streamplaceUrl) {
    throw new Error("streamplaceUrl is required");
  }

  let customResolver: StreamplaceOAuthResolver | null = null;

  let meta: ClientMetadata;

  const redirectURI = `${streamplaceUrl}${getCallbackUrl()}`;

  const res = await fetch(
    `${streamplaceUrl}/oauth/downstream/client-metadata.json?redirect_uri=${encodeURIComponent(redirectURI)}`,
  );
  meta = await res.json();

  if (
    streamplaceUrl.startsWith("http://localhost") ||
    streamplaceUrl.startsWith("http://127.0.0.1")
  ) {
    if (!meta.scope) {
      throw new Error("meta.scope is required");
    }
    const u = new URL(streamplaceUrl);
    let hostname = u.hostname;
    if (hostname === "localhost") {
      hostname = "127.0.0.1";
    }
    let redirect = `${u.protocol}//${hostname}`;
    if (u.port !== "") {
      redirect = `${redirect}:${u.port}`;
    }
    redirect = `${redirect}${getCallbackUrl()}`;

    const queryParams = new URLSearchParams();
    queryParams.set("scope", meta.scope);
    queryParams.set("redirect_uri", redirect);
    meta = {
      client_id: `http://localhost?${queryParams.toString()}`,
      redirect_uris: [redirect as any],
      scope: meta.scope,
      token_endpoint_auth_method: "none",
      client_name: "Loopback client",
      response_types: ["code"],
      grant_types: ["authorization_code", "refresh_token"],
      application_type: "native",
      dpop_bound_access_tokens: true,
      subject_type: "public",
      authorization_signed_response_alg: "ES256",
    };
  }

  try {
    clientMetadataSchema.parse(meta);
  } catch {
    throw new Error("Invalid OAuth client metadata from server");
  }

  const client = new BrowserOAuthClient({
    fetch: async (input, init) => {
      let request: Request;
      if (typeof input === "string" || input instanceof URL) {
        request = new Request(input, init);
      } else {
        request = input;
      }

      if (
        customResolver &&
        request.url.includes("/oauth/par") &&
        request.method === "POST"
      ) {
        const resourceServer = customResolver.getCurrentResourceServer();
        if (resourceServer) {
          const clonedRequest = request.clone();
          const body = await clonedRequest.text();
          const params = new URLSearchParams(body);
          params.set("login_hint", resourceServer);
          request = new Request(request.url, {
            method: request.method,
            headers: request.headers,
            body: params.toString(),
          });
        }
      }

      if (streamplaceUrl.startsWith("http://127.0.0.1")) {
        if (
          request.url.includes("plc.directory") ||
          request.url.endsWith("did.json") ||
          request.url.endsWith("/.well-known/oauth-protected-resource") ||
          request.url.endsWith("/.well-known/oauth-authorization-server")
        ) {
          return fetch(request, init);
        }
        const newUrl = new URL(request.url.toString());
        newUrl.protocol = "http:";
        newUrl.host = "127.0.0.1:38080";
        let newRequest: Request;
        if (request.method === "POST") {
          const data = await request.blob();
          newRequest = new Request(newUrl.toString(), {
            body: data,
            method: "POST",
            headers: request.headers,
          });
        } else if (request.method === "GET") {
          newRequest = new Request(newUrl.toString(), {
            method: "GET",
            headers: request.headers,
          });
        } else {
          throw new Error("Unsupported method: " + request.method);
        }
        return fetch(newRequest);
      } else {
        if (
          request.url.includes("plc.directory") ||
          request.url.endsWith("did.json")
        ) {
          const res = await fetch(request, init);
          if (!res.ok) {
            return res;
          }
          const data = await res.json();
          const service = data.service?.find(
            (s: { id: string; serviceEndpoint: string }) =>
              s.id === "#atproto_pds",
          );
          if (!service) {
            return res;
          }
          service.serviceEndpoint = streamplaceUrl;
          return new Response(JSON.stringify(data), {
            status: res.status,
            headers: res.headers,
          });
        } else {
          return fetch(request, init);
        }
      }
    },
    handleResolver: streamplaceUrl,
    responseMode: "query",
    clientMetadata: meta,
  });

  // Replace the default resolver with our downstream-OAuth resolver.
  customResolver = new StreamplaceOAuthResolver(streamplaceUrl);
  // The library types oauthResolver as a readonly OAuthResolver, but at
  // runtime it's a plain property. We duck-type our resolver to match
  // the shape the library needs. Cast through unknown to bypass the
  // type mismatch (OAuthResolver vs StreamplaceOAuthResolver).
  if ("oauthResolver" in client) {
    (
      client as unknown as { oauthResolver: StreamplaceOAuthResolver }
    ).oauthResolver = customResolver;
  }

  return client;
}
