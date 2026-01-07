import {
  ClientMetadata,
  clientMetadataSchema,
  ReactNativeOAuthClient,
} from "@streamplace/atproto-oauth-client-react-native";
import Constants from "expo-constants";
import { Platform } from "react-native";
import { StreamplaceOAuthResolver } from "./oauthResolver";

export type StreamplaceOAuthClient = Omit<
  ReactNativeOAuthClient,
  "keyset" | "serverFactory" | "jwks"
>;

export default async function createOAuthClient(
  streamplaceUrl: string,
): Promise<StreamplaceOAuthClient> {
  if (!streamplaceUrl) {
    throw new Error("streamplaceUrl is required");
  }

  // Will be set after we create the custom resolver
  let customResolver: StreamplaceOAuthResolver | null = null;

  let meta: ClientMetadata;
  if (
    streamplaceUrl.startsWith("http://localhost") ||
    streamplaceUrl.startsWith("http://127.0.0.1")
  ) {
    const isWeb = Platform.OS === "web";
    const u = new URL(streamplaceUrl);
    let hostname = u.hostname;
    if (hostname == "localhost") {
      hostname = "127.0.0.1";
    }
    let redirect = `${u.protocol}//${hostname}`;
    if (u.port !== "") {
      redirect = `${redirect}:${u.port}`;
    }
    if (isWeb) {
      redirect = `${redirect}/login`;
    } else {
      const scheme = Constants.expoConfig?.scheme;
      if (!scheme) {
        throw new Error("unable to resolve scheme for oauth redirect");
      }
      redirect = `${redirect}/app-return/${scheme}`;
    }
    const queryParams = new URLSearchParams();
    queryParams.set("scope", "atproto transition:generic");
    queryParams.set("redirect_uri", redirect);
    meta = {
      client_id: `http://localhost?${queryParams.toString()}`,
      redirect_uris: [redirect as any],
      scope: "atproto transition:generic",
      token_endpoint_auth_method: "none",
      client_name: "Loopback client",
      response_types: ["code"],
      grant_types: ["authorization_code", "refresh_token"],
      // > There is a special exception for the localhost development workflow [ ... ]
      // > These clients use web URLs, but have application_type set to native in the generated client metadata.
      application_type: "native",
      dpop_bound_access_tokens: true,
      subject_type: "public",
      authorization_signed_response_alg: "ES256",
    };
  } else {
    const redirectURI =
      Platform.OS === "web"
        ? `${streamplaceUrl}/login`
        : `${streamplaceUrl}/api/app-return`;
    const res = await fetch(
      `${streamplaceUrl}/oauth/downstream/client-metadata.json?redirect_uri=${encodeURIComponent(redirectURI)}`,
    );
    meta = await res.json();
  }
  try {
    clientMetadataSchema.parse(meta);
  } catch (e) {
    console.error("error parsing client metadata", e, meta);
    throw e;
  }
  const client = new ReactNativeOAuthClient({
    fetch: async (input, init) => {
      // Normalize input to a Request object
      let request: Request;
      if (typeof input === "string" || input instanceof URL) {
        request = new Request(input, init);
      } else {
        request = input;
      }

      // Add login_hint parameter to PAR requests
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
        // everything other than PDS resolution gets rewritten to the host
        if (
          request.url.includes("plc.directory") ||
          request.url.endsWith("did.json") ||
          request.url.endsWith("/.well-known/oauth-protected-resource") ||
          request.url.endsWith("/.well-known/oauth-authorization-server")
        ) {
          return fetch(request, init) as any;
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
        return fetch(newRequest) as any;
      } else {
        // Lie to the oauth client and use our upstream server instead
        if (
          request.url.includes("plc.directory") ||
          request.url.endsWith("did.json")
        ) {
          const res = await fetch(request, init);
          if (!res.ok) {
            return res;
          }
          const data = await res.json();
          const service = data.service.find(
            (s: any) => s.id === "#atproto_pds",
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
    responseMode: "query", // or "fragment" (frontend only) or "form_post" (backend only)

    // These must be the same metadata as the one exposed on the
    // "client_id" endpoint (except when using a loopback client)
    clientMetadata: meta,
  });

  // Replace the default OAuth resolver with our custom one
  customResolver = new StreamplaceOAuthResolver(
    streamplaceUrl,
    client.oauthResolver.identityResolver,
    client.oauthResolver.protectedResourceMetadataResolver,
    client.oauthResolver.authorizationServerMetadataResolver,
  );
  // @ts-ignore override readonly property
  client.oauthResolver = customResolver;

  return client;
}
