import { useCallback, useEffect, useState } from "react";

const PUBLIC_APPVIEW = "https://public.api.bsky.app";

type PublicActor = {
  did: string;
  handle: string;
};

export type ActorLookupResult =
  | { status: "found"; actor: PublicActor }
  | { status: "not-found" };

type FetchActor = (
  input: RequestInfo | URL,
  init?: RequestInit,
) => Promise<Response>;

export async function resolvePublicActor(
  actor: string,
  fetchActor: FetchActor = fetch,
  signal?: AbortSignal,
): Promise<ActorLookupResult> {
  const response = await fetchActor(
    `${PUBLIC_APPVIEW}/xrpc/app.bsky.actor.getProfile?actor=${encodeURIComponent(actor)}`,
    { signal },
  );

  if (response.ok) {
    const profile = (await response.json()) as PublicActor;
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
    resolvePublicActor(actor, fetch, controller.signal)
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
