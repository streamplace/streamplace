import { useEffect, useState } from "react";
import { useStreamplaceStore } from "../streamplace-store";
import { useLivestreamStore } from "./livestream-store";

interface DIDDoc {
  "@context": string[];
  id: string;
  alsoKnownAs: string[];
  service: unknown[];
  verificationMethod: unknown[];
  assertionMethod: string[];
}

export const usePublisherKey = (): {
  publisherKey: string | null;
  error: string | null;
  loading: boolean;
} => {
  const streamplaceUrl = useStreamplaceStore((state) => state.url);
  const publisherKey = useLivestreamStore((state) => state.publisherKey);
  const setPublisherKey = useLivestreamStore((state) => state.setPublisherKey);
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(false);

  useEffect(() => {
    if (publisherKey) {
      return;
    }

    const fetchPublisherKey = async () => {
      setLoading(true);
      setError(null);

      try {
        const response = await fetch(`${streamplaceUrl}/.well-known/did.json`);
        if (!response.ok) {
          throw new Error(`Failed to fetch DID doc: ${response.statusText}`);
        }

        const didDoc: DIDDoc = await response.json();

        if (!didDoc.assertionMethod || didDoc.assertionMethod.length === 0) {
          throw new Error("No publisher key found in DID document");
        }

        const key = didDoc.assertionMethod[0];
        setPublisherKey(key);
      } catch (err) {
        const message = err instanceof Error ? err.message : "Unknown error";
        setError(message);
      } finally {
        setLoading(false);
      }
    };

    fetchPublisherKey();
  }, [streamplaceUrl, publisherKey, setPublisherKey]);

  return { publisherKey, error, loading };
};
