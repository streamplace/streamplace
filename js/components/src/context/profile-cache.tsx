import { ProfileViewDetailed } from "@atproto/api/dist/client/types/app/bsky/actor/defs";
import { createContext, useContext, useRef, useState } from "react";

interface ProfileCacheContextValue {
  profiles: Record<string, ProfileViewDetailed>;
  inFlight: React.MutableRefObject<Set<string>>;
  addProfiles: (profiles: Record<string, ProfileViewDetailed>) => void;
}

export const ProfileCacheContext =
  createContext<ProfileCacheContextValue | null>(null);

export function ProfileCacheProvider({
  children,
}: {
  children: React.ReactNode;
}) {
  const [profiles, setProfiles] = useState<Record<string, ProfileViewDetailed>>(
    {},
  );
  const inFlight = useRef<Set<string>>(new Set());

  const addProfiles = (newProfiles: Record<string, ProfileViewDetailed>) =>
    setProfiles((prev) => ({ ...prev, ...newProfiles }));

  return (
    <ProfileCacheContext.Provider value={{ profiles, inFlight, addProfiles }}>
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
