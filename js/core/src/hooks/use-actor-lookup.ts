// Confirms an account exists via the configured node's describeRepo, which
// accepts handles and DIDs and doesn't require a Bluesky profile.
import { useCallback, useEffect, useState } from "react";

type RepoActor = {
  did: string;
  handle: string;
};

export type ActorLookupResult =
  { status: "found"; actor: RepoActor } | { status: "not-found" };

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
  const response = await fetchActor(
    `${nodeUrl.replace(/\/+$/, "")}/xrpc/com.atproto.repo.describeRepo?repo=${encodeURIComponent(actor)}`,
    { signal },
  );

  if (response.ok) {
    const repo = (await response.json()) as Partial<RepoActor> | null;
    if (typeof repo?.did !== "string" || typeof repo.handle !== "string") {
      throw new Error("Repo lookup returned a malformed response");
    }
    return { status: "found", actor: { did: repo.did, handle: repo.handle } };
  }

  const body = (await response.json().catch(() => null)) as {
    error?: string;
    message?: string;
  } | null;
  // The node answers 404 "RepoNotFound"; upstream PDSes answer 400 with that XRPC error.
  if (
    (response.status === 404 &&
      (body?.message === "RepoNotFound" || body?.error === "RepoNotFound")) ||
    (response.status === 400 && body?.error === "RepoNotFound")
  ) {
    return { status: "not-found" };
  }

  throw new Error(`Repo lookup failed (${response.status})`);
}

type ActorLookupState = ActorLookupResult | { status: "error" };

export function useActorLookup(actor: string, nodeUrl: string) {
  const [attempt, setAttempt] = useState(0);
  // Results are keyed to their request so a changed actor, node, or retry reads as loading.
  const key = `${nodeUrl} ${actor} ${attempt}`;
  const [settled, setSettled] = useState<{
    key: string;
    state: ActorLookupState;
  } | null>(null);

  useEffect(() => {
    const controller = new AbortController();
    const settle = (state: ActorLookupState) => {
      if (!controller.signal.aborted) setSettled({ key, state });
    };
    resolveActor(actor, nodeUrl, fetch, controller.signal).then(settle, () =>
      settle({ status: "error" }),
    );
    return () => controller.abort();
  }, [actor, nodeUrl, key]);

  const retry = useCallback(() => setAttempt((value) => value + 1), []);

  const state: ActorLookupState | { status: "loading" } =
    settled?.key === key ? settled.state : { status: "loading" };
  return { ...state, retry };
}
