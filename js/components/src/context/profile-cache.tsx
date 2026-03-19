import { AppBskyActorDefs } from "@atproto/api";
import {
  createContext,
  useCallback,
  useContext,
  useMemo,
  useRef,
  useState,
} from "react";
import { useUnauthenticatedBlueskyAppViewAgent } from "../streamplace-store";

interface ProfileCacheContextValue {
  profiles: Record<string, AppBskyActorDefs.ProfileViewDetailed>;
  requestProfiles: (dids: string[]) => void;
}

export const ProfileCacheContext =
  createContext<ProfileCacheContextValue | null>(null);

export function ProfileCacheProvider({
  children,
}: {
  children: React.ReactNode;
}) {
  const agent = useUnauthenticatedBlueskyAppViewAgent();
  const [profiles, setProfiles] = useState<
    Record<string, AppBskyActorDefs.ProfileViewDetailed>
  >({});
  const agentRef = useRef(agent);
  agentRef.current = agent;
  const profilesRef = useRef(profiles);
  profilesRef.current = profiles;
  const inFlight = useRef<Set<string>>(new Set());
  const pending = useRef<Set<string>>(new Set());
  const timer = useRef<ReturnType<typeof setTimeout> | null>(null);

  const flush = useCallback(() => {
    timer.current = null;
    if (!agentRef.current) return;

    const toFetch = [...pending.current]
      .filter((d) => !(d in profilesRef.current) && !inFlight.current.has(d))
      .slice(0, 25);
    toFetch.forEach((d) => pending.current.delete(d));

    if (toFetch.length === 0) return;

    // If there are more beyond the batch limit, schedule the remainder
    if (pending.current.size > 0) {
      timer.current = setTimeout(flush, 0);
    }

    toFetch.forEach((d) => inFlight.current.add(d));

    agentRef.current
      .getProfiles({ actors: toFetch })
      .then((result) => {
        const newProfiles: Record<
          string,
          AppBskyActorDefs.ProfileViewDetailed
        > = {};
        result.data.profiles.forEach((p) => {
          newProfiles[p.did] = p;
        });
        setProfiles((prev) => ({ ...prev, ...newProfiles }));
      })
      .catch((e) => {
        console.error("Failed to fetch profiles", e);
      })
      .finally(() => {
        toFetch.forEach((d) => inFlight.current.delete(d));
      });
  }, []);

  const requestProfiles = useCallback(
    (dids: string[]) => {
      const toQueue = dids.filter(
        (d) => !(d in profilesRef.current) && !inFlight.current.has(d),
      );
      if (toQueue.length === 0) return;
      toQueue.forEach((d) => pending.current.add(d));
      if (!timer.current) {
        timer.current = setTimeout(flush, 50);
      }
    },
    [flush],
  );

  const value = useMemo(
    () => ({ profiles, requestProfiles }),
    [profiles, requestProfiles],
  );

  return (
    <ProfileCacheContext.Provider value={value}>
      {children}
    </ProfileCacheContext.Provider>
  );
}

export function useProfileCache(): ProfileCacheContextValue {
  const ctx = useContext(ProfileCacheContext);
  if (!ctx) {
    throw new Error("useProfileCache must be used within ProfileCacheProvider");
  }
  return ctx;
}
