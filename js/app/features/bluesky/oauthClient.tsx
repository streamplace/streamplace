import Constants from "expo-constants";
import { Platform } from "react-native";
import {
  ClientMetadata,
  clientMetadataSchema,
  OAuthClient,
} from "./oauthClientImport";
import { routeOAuthThroughStreamplace } from "./oauthResolver";

export default async function createOAuthClient(
  streamplaceUrl: string,
): Promise<OAuthClient> {
  console.log("createOAuthClient", streamplaceUrl);
  if (!streamplaceUrl) {
    throw new Error("streamplaceUrl is required");
  }

  let getLoginHint: (() => string | null) | null = null;

  let meta: ClientMetadata;

  const nodeUrl = new URL(streamplaceUrl);
  const isLoopback =
    nodeUrl.protocol === "http:" &&
    (nodeUrl.hostname === "localhost" ||
      nodeUrl.hostname === "127.0.0.1" ||
      nodeUrl.hostname === "[::1]");
  const callbackUrl = new URL(nodeUrl.toString());
  if (isLoopback && callbackUrl.hostname === "localhost") {
    callbackUrl.hostname = "127.0.0.1";
  }
  if (Platform.OS === "web") {
    callbackUrl.pathname = "/login";
  } else if (isLoopback) {
    const scheme = Constants.expoConfig?.scheme;
    if (!scheme) {
      throw new Error("unable to resolve scheme for oauth redirect");
    }
    callbackUrl.pathname = `/app-return/${scheme}`;
  } else {
    callbackUrl.pathname = "/api/app-return";
  }
  const redirectURI = callbackUrl.toString();
  const res = await fetch(
    `${streamplaceUrl}/oauth/downstream/client-metadata.json?redirect_uri=${encodeURIComponent(redirectURI)}`,
  );
  meta = await res.json();

  // Legacy public dev nodes advertise a HTTPS redirect wrapper even for their
  // HTTP loopback client. New local nodes advertise the actual redirect.
  if (
    isLoopback &&
    meta.client_id?.startsWith("http://localhost?") &&
    !meta.redirect_uris?.some((uri) => uri === redirectURI)
  ) {
    if (!meta.scope) {
      throw new Error("meta.scope is required");
    }
    const queryParams = new URLSearchParams();
    queryParams.set("scope", meta.scope);
    queryParams.set("redirect_uri", redirectURI);
    meta = {
      ...meta,
      client_id: `http://localhost?${queryParams.toString()}`,
      redirect_uris: [redirectURI as ClientMetadata["redirect_uris"][number]],
      application_type: "native",
    };
  }
  try {
    clientMetadataSchema.parse(meta);
  } catch (e) {
    console.error("error parsing client metadata", e, meta);
    throw e;
  }
  const client = new OAuthClient({
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
        getLoginHint &&
        request.url.includes("/oauth/par") &&
        request.method === "POST"
      ) {
        const resourceServer = getLoginHint();
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

      return fetch(request, init);
    },
    allowHttp: isLoopback,
    handleResolver: streamplaceUrl,
    responseMode: "query", // or "fragment" (frontend only) or "form_post" (backend only)

    // These must be the same metadata as the one exposed on the
    // "client_id" endpoint (except when using a loopback client)
    clientMetadata: meta,
  });

  getLoginHint = routeOAuthThroughStreamplace(
    client.oauthResolver,
    streamplaceUrl,
  );

  return client;
}
