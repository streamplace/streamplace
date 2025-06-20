import { bytesToMultibase, Secp256k1Keypair } from "@atproto/crypto";
import { useEffect, useState } from "react";
import { Platform } from "react-native";
import { PlaceStreamKey } from "streamplace";
import { privateKeyToAccount } from "viem/accounts";
import { usePDSAgent } from "../streamplace-store/xrpc";
import { useLivestreamStore } from "./livestream-store";

function getBrowserName(userAgent: string) {
  // The order matters here, and this may report false positives for unlisted browsers.

  if (userAgent.includes("Firefox")) {
    // "Mozilla/5.0 (X11; Linux i686; rv:104.0) Gecko/20100101 Firefox/104.0"
    return "Mozilla Firefox";
  } else if (userAgent.includes("SamsungBrowser")) {
    // "Mozilla/5.0 (Linux; Android 9; SAMSUNG SM-G955F Build/PPR1.180610.011) AppleWebKit/537.36 (KHTML, like Gecko) SamsungBrowser/9.4 Chrome/67.0.3396.87 Mobile Safari/537.36"
    return "Samsung Internet";
  } else if (userAgent.includes("Opera") || userAgent.includes("OPR")) {
    // "Mozilla/5.0 (Macintosh; Intel Mac OS X 12_5_1) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/104.0.0.0 Safari/537.36 OPR/90.0.4480.54"
    return "Opera";
  } else if (userAgent.includes("Edge")) {
    // "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/58.0.3029.110 Safari/537.36 Edge/16.16299"
    return "Microsoft Edge (Legacy)";
  } else if (userAgent.includes("Edg")) {
    // "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/104.0.0.0 Safari/537.36 Edg/104.0.1293.70"
    return "Microsoft Edge (Chromium)";
  } else if (userAgent.includes("Chrome")) {
    // "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/104.0.0.0 Safari/537.36"
    return "Google Chrome or Chromium";
  } else if (userAgent.includes("Safari")) {
    // "Mozilla/5.0 (iPhone; CPU iPhone OS 15_6_1 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/15.6 Mobile/15E148 Safari/604.1"
    return "Apple Safari";
  }
  return "unknown";
}

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

  useEffect(() => {
    if (key) return; // already have key

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
        signingKey: keypair.did(),
        createdAt: new Date().toISOString(),
        createdBy: "Streamplace on " + platform,
      };
      await pdsAgent.com.atproto.repo.createRecord({
        repo: did,
        collection: "place.stream.key",
        record,
      });

      setStreamKey(JSON.stringify(newKey));
      setKey(newKey);
    };

    generateKey();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [key, setStreamKey]);

  return { streamKey: key, error };
};
