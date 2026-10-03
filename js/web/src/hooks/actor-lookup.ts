import { getStreamplaceUrl } from "@/lib/streamplace-url";
import { useCallback, useEffect, useState } from "react";

const PUBLIC_APPVIEW = "https://public.api.bsky.app";

type Actor = {
  did: string;
  handle: string;
};

export type ActorLookupResult =
  | { status: "found"; actor: Actor }
  | { status: "not-found" };

type FetchActor = (
  input: RequestInfo | URL,
  init?: RequestInit,
) => Promise<Response>;

export async function resolveActor(
  actor: string,
  nodeUrl: string,
  fetchActor: FetchActor = fetch,
  signal?: AbortSignal,
): Promise<ActorLookupResult> {
  const indexed = await fetchActor(
    `${nodeUrl}/api/livestream/${encodeURIComponent(actor)}`,
    { signal },
  );
  if (indexed.ok) {
    const livestream = (await indexed.json()) as { author: Actor };
    return {
      status: "found",
      actor: { did: livestream.author.did, handle: livestream.author.handle },
    };
  }
  if (indexed.status !== 404) {
    throw new Error(`Profile lookup failed (${indexed.status})`);
  }

  const response = await fetchActor(
    `${PUBLIC_APPVIEW}/xrpc/app.bsky.actor.getProfile?actor=${encodeURIComponent(actor)}`,
    { signal },
  );

  if (response.ok) {
    const profile = (await response.json()) as Actor;
    return {
      status: "found",
      actor: { did: profile.did, handle: profile.handle },
    };
  }

  const body = (await response.json().catch(() => null)) as {
    message?: string;
  } | null;
  if (
    (response.status === 400 || response.status === 404) &&
    body?.message?.toLowerCase() === "profile not found"
  ) {
    return { status: "not-found" };
  }

  throw new Error(`Profile lookup failed (${response.status})`);
}

type ActorLookupState =
  | { status: "loading" }
  | ActorLookupResult
  | { status: "error" };

export function useActorLookup(actor: string) {
  const [attempt, setAttempt] = useState(0);
  const [state, setState] = useState<ActorLookupState>({ status: "loading" });

  useEffect(() => {
    const controller = new AbortController();
    setState({ status: "loading" });
    resolveActor(actor, getStreamplaceUrl(), fetch, controller.signal)
      .then((result) => setState(result))
      .catch((error) => {
        if (error instanceof DOMException && error.name === "AbortError") {
          return;
        }
        setState({ status: "error" });
      });
    return () => controller.abort();
  }, [actor, attempt]);

  const retry = useCallback(() => setAttempt((value) => value + 1), []);

  return { ...state, retry };
}
