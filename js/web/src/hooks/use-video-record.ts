// Fetches a single video record by at:// URI for the VOD page.
import { useEffect, useState } from "react";
import type { PlaceStreamVideo } from "streamplace";
import { useStore } from "../lib/store";

export function useVideoRecord(user: string, tid: string) {
  const streamplaceUrl = useStore((state) => state.url);
  const anonPDSAgent = useStore((state) => state.anonPDSAgent);
  const [record, setRecord] = useState<PlaceStreamVideo.Record | null>(null);
  const [author, setAuthor] = useState<{
    did: string;
    handle?: string;
    displayName?: string;
    avatar?: string;
  } | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    let cancelled = false;

    async function fetchRecord() {
      setLoading(true);
      setError(null);

      try {
        // Use the anonPDSAgent if available, otherwise fetch directly.
        let data: any;
        if (anonPDSAgent) {
          const res = await anonPDSAgent.api.com.atproto.repo.getRecord({
            repo: user,
            collection: "place.stream.video",
            rkey: tid,
          });
          data = res.data;
        } else {
          const uri = `at://${user}/place.stream.video/${tid}`;
          const res = await fetch(
            `${streamplaceUrl}/xrpc/com.atproto.repo.getRecord?repo=${encodeURIComponent(user)}&collection=place.stream.video&rkey=${tid}`,
          );
          if (!res.ok) throw new Error(`Failed to fetch video: ${res.status}`);
          data = await res.json();
        }

        if (cancelled) return;
        setRecord(data.value as PlaceStreamVideo.Record);

        // Try to get profile info from the repo itself.
        // The AT Protocol getRecord doesn't include author info, so we
        // do a lightweight resolveHandle + getProfile if available.
        try {
          if (anonPDSAgent) {
            const profile = await anonPDSAgent.getProfile({ actor: user });
            if (!cancelled) {
              setAuthor({
                did: profile.data.did,
                handle: profile.data.handle,
                displayName: profile.data.displayName,
                avatar: profile.data.avatar,
              });
            }
          } else {
            // If no agent, we can still fetch the profile via the public API.
            const profileRes = await fetch(
              `${streamplaceUrl}/xrpc/app.bsky.actor.getProfile?actor=${encodeURIComponent(user)}`,
            );
            if (profileRes.ok) {
              const profileData = await profileRes.json();
              if (!cancelled) {
                setAuthor({
                  did: profileData.did,
                  handle: profileData.handle,
                  displayName: profileData.displayName,
                  avatar: profileData.avatar,
                });
              }
            }
          }
        } catch {
          // Profile fetch is best-effort; the page still works without it.
        }
      } catch (e: any) {
        if (!cancelled) {
          setError(e?.message ?? "Failed to load video");
        }
      } finally {
        if (!cancelled) setLoading(false);
      }
    }

    fetchRecord();
    return () => {
      cancelled = true;
    };
  }, [user, tid, anonPDSAgent, streamplaceUrl]);

  return { record, author, loading, error };
}
