import {
  ClientMetadata,
  clientMetadataSchema,
  ReactNativeOAuthClient,
} from "@aquareum/atproto-oauth-client-react-native";
import Constants from "expo-constants";
import { Platform } from "react-native";

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
  let meta: ClientMetadata;
  // if (
  //   streamplaceUrl.startsWith("http://localhost") ||
  //   streamplaceUrl.startsWith("http://127.0.0.1")
  // ) {
  //   const isWeb = Platform.OS === "web";
  //   const u = new URL(streamplaceUrl);
  //   let hostname = u.hostname;
  //   if (hostname == "localhost") {
  //     hostname = "127.0.0.1";
  //   }
  //   let redirect = `${u.protocol}//${hostname}`;
  //   if (u.port !== "") {
  //     redirect = `${redirect}:${u.port}`;
  //   }
  //   if (isWeb) {
  //     redirect = `${redirect}/login`;
  //   } else {
  //     const scheme = Constants.expoConfig?.scheme;
  //     if (!scheme) {
  //       throw new Error("unable to resolve scheme for oauth redirect");
  //     }
  //     redirect = `${redirect}/app-return/${scheme}`;
  //   }
  //   const queryParams = new URLSearchParams();
  //   queryParams.set("scope", "atproto transition:generic");
  //   queryParams.set("redirect_uri", redirect);
  //   meta = {
  //     client_id: `http://localhost?${queryParams.toString()}`,
  //     redirect_uris: [redirect as any],
  //     scope: "atproto transition:generic",
  //     token_endpoint_auth_method: "none",
  //     client_name: "Loopback client",
  //     response_types: ["code"],
  //     grant_types: ["authorization_code", "refresh_token"],
  //     // > There is a special exception for the localhost development workflow [ ... ]
  //     // > These clients use web URLs, but have application_type set to native in the generated client metadata.
  //     application_type: "native",
  //     dpop_bound_access_tokens: true,
  //   };
  // } else {
  //   const res = await fetch(
  //     `${streamplaceUrl}/api/atproto-oauth/${Platform.OS}`,
  //   );
  //   meta = await res.json();
  // }
  meta = {
    redirect_uris: [
      "https://longos.iameli.link/xrpc/place.stream.account.oauthReturn",
    ],
    response_types: ["code"],
    grant_types: ["authorization_code", "refresh_token"],
    scope: "atproto transition:generic",
    token_endpoint_auth_method: "none",
    application_type: "web",
    client_id: "https://longos.iameli.link/api/atproto-oauth/web",
    client_name: "Streamplace",
    client_uri: "https://longos.iameli.link",
    dpop_bound_access_tokens: true,
  };
  clientMetadataSchema.parse(meta);
  return new ReactNativeOAuthClient({
    fetch: (input, init) => {
      console.log("!!!!!! fetch", input, init);
      return fetch(input, init);
    },
    handleResolver: "https://bsky.social", // backend instances should use a DNS based resolver
    responseMode: "query", // or "fragment" (frontend only) or "form_post" (backend only)

    // These must be the same metadata as the one exposed on the
    // "client_id" endpoint (except when using a loopback client)
    clientMetadata: meta,
  });
}
