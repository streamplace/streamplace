import {
  ReactNativeOAuthClient,
  ReactNativeOAuthClientOptions,
} from "@atproto/oauth-client-react-native";
import { Platform } from "react-native";

export default async function createOAuthClient(aquareumUrl: string) {
  if (!aquareumUrl) {
    throw new Error("aquareumUrl is required");
  }
  let meta: typeof ReactNativeOAuthClient.prototype.clientMetadata;
  const res = await fetch(`${aquareumUrl}/api/atproto-oauth/${Platform.OS}`);
  meta = await res.json();
  const opts: ReactNativeOAuthClientOptions = {
    handleResolver: "https://bsky.social",
    responseMode: "query",
    clientMetadata: meta,
  };
  return new ReactNativeOAuthClient(opts);
}
