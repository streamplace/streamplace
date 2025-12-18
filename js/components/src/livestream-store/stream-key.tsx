import { bytesToMultibase, Secp256k1Keypair } from "@atproto/crypto";
import { useEffect, useState } from "react";
import { Platform } from "react-native";
import { PlaceStreamKey } from "streamplace";
import { privateKeyToAccount } from "viem/accounts";
import { getBrowserName } from "../lib/browser";
import { usePDSAgent } from "../streamplace-store/xrpc";
import { useLivestreamStore } from "./livestream-store";
import { usePublisherKey } from "./use-publisher-key";

export const useStreamKey = (): {
  streamKey: {
    privateKey: string;
    did: string;
    address: string;
  } | null;
  error: string | null;
} => {
  const pdsAgent = usePDSAgent();
  const streamKey = useLivestreamStore((state) => state.streamKey);
  const setStreamKey = useLivestreamStore((state) => state.setStreamKey);
  const [key, setKey] = useState<any>(streamKey ? JSON.parse(streamKey) : null);
  const [error, setError] = useState<string | null>(null);
  const {
    publisherKey,
    error: publisherKeyError,
    loading: publisherKeyLoading,
  } = usePublisherKey();

  useEffect(() => {
    if (key) return;

    if (publisherKeyLoading) return;

    if (publisherKeyError) {
      setError(`Failed to fetch publisher key (required for stream key record): ${publisherKeyError}`);
      return;
    }

    if (!publisherKey) return;

    const generateKey = async () => {
      if (!pdsAgent) {
        setError("PDS Agent is not available");
        return;
      }
      let did = pdsAgent.did;
      if (!did) {
        setError("PDS Agent did is not available (not logged in?)");
        return;
      }

      const keypair = await Secp256k1Keypair.create({ exportable: true });
      const exportedKey = await keypair.export();
      const didBytes = new TextEncoder().encode(did);
      const combinedKey = new Uint8Array([...exportedKey, ...didBytes]);
      const multibaseKey = bytesToMultibase(combinedKey, "base58btc");
      const hexKey = Array.from(exportedKey)
        .map((b) => b.toString(16).padStart(2, "0"))
        .join("");
      const account = privateKeyToAccount(`0x${hexKey}`);
      const newKey = {
        privateKey: multibaseKey,
        did: keypair.did(),
        address: account.address.toLowerCase(),
      };

      let platform: string = Platform.OS;
      if (
        Platform.OS === "web" &&
        typeof window !== "undefined" &&
        window.navigator
      ) {
        if (window.navigator.userAgent.includes("streamplace-desktop")) {
          platform = "Desktop";
        } else {
          platform = getBrowserName(window.navigator.userAgent);
          if (platform !== "unknown") {
            platform = platform;
          }
        }
      } else if (platform === "android") {
        platform = "Android";
      } else if (platform === "ios") {
        platform = "iOS";
      } else if (platform === "macos") {
        platform = "macOS";
      } else if (platform === "windows") {
        platform = "Windows";
      }

      const record: PlaceStreamKey.Record = {
        $type: "place.stream.key",
        signingKey: keypair.did(),
        createdAt: new Date().toISOString(),
        createdBy: "Streamplace on " + platform,
        publisher: publisherKey,
      };
      await pdsAgent.com.atproto.repo.createRecord({
        repo: did,
        collection: "place.stream.key",
        record,
      });

      setStreamKey(JSON.stringify(newKey));
      setKey(newKey);
    };

    generateKey().catch((err) => setError(err.message));
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [key, setStreamKey]);

  return { streamKey: key, error };
};
