import { useEffect, useRef, useState } from "react";

interface Actor {
  did: string;
  handle: string;
  displayName?: string;
  avatar?: string;
}

interface TypeaheadResult {
  actors: Actor[];
  loading: boolean;
  error: string | null;
}

const MIN_QUERY_LENGTH = 3;
const DEBOUNCE_MS = 500;
const MIN_REQUEST_INTERVAL_MS = 1000;
const RESULT_LIMIT = 3;

export default function useActorTypeahead(query: string): TypeaheadResult {
  const [actors, setActors] = useState<Actor[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const abortControllerRef = useRef<AbortController | null>(null);
  const lastRequestTimeRef = useRef<number>(0);
  const debounceTimerRef = useRef<NodeJS.Timeout | null>(null);
  const actorsRef = useRef<Actor[]>([]);

  useEffect(() => {
    if (debounceTimerRef.current) {
      clearTimeout(debounceTimerRef.current);
    }

    if (abortControllerRef.current) {
      abortControllerRef.current.abort();
    }

    if (query.length < MIN_QUERY_LENGTH) {
      setActors([]);
      setLoading(false);
      setError(null);
      return;
    }

    setLoading(true);
    setError(null);

    debounceTimerRef.current = setTimeout(async () => {
      const now = Date.now();
      const timeSinceLastRequest = now - lastRequestTimeRef.current;

      if (timeSinceLastRequest < MIN_REQUEST_INTERVAL_MS) {
        const delay = MIN_REQUEST_INTERVAL_MS - timeSinceLastRequest;
        await new Promise((resolve) => setTimeout(resolve, delay));
      }

      lastRequestTimeRef.current = Date.now();

      const controller = new AbortController();
      abortControllerRef.current = controller;

      try {
        const params = new URLSearchParams({
          q: query,
          limit: RESULT_LIMIT.toString(),
        });

        const response = await fetch(
          `https://public.api.bsky.app/xrpc/app.bsky.actor.searchActorsTypeahead?${params.toString()}`,
          { signal: controller.signal },
        );

        if (!response.ok) {
          throw new Error(`API error: ${response.status}`);
        }

        const data = await response.json();

        if (!controller.signal.aborted) {
          const newActors = data.actors || [];

          // check if actors actually changed
          const actorsChanged =
            newActors.length !== actorsRef.current.length ||
            newActors.some(
              (actor: Actor, i: number) =>
                actor.did !== actorsRef.current[i]?.did ||
                actor.avatar !== actorsRef.current[i]?.avatar,
            );

          if (actorsChanged) {
            actorsRef.current = newActors;
            setActors(newActors);
          } else {
            // keep the same reference to prevent re-renders
            setActors(actorsRef.current);
          }
          setLoading(false);
        }
      } catch (err: any) {
        if (err.name !== "AbortError" && !controller.signal.aborted) {
          setError(err.message || "Failed to search actors");
          setActors([]);
          setLoading(false);
        }
      }
    }, DEBOUNCE_MS);

    return () => {
      if (debounceTimerRef.current) {
        clearTimeout(debounceTimerRef.current);
      }
      if (abortControllerRef.current) {
        abortControllerRef.current.abort();
      }
    };
  }, [query]);

  return { actors, loading, error };
}
