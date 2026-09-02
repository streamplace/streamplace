import type { LivestreamStore } from "@streamplace/core";
import { useMemo } from "react";
import { useStore } from "zustand";
import { useShallow } from "zustand/react/shallow";
import { resolveStreamAvatar } from "./resolve-stream-avatar";
import useAvatars from "./use-avatars";

export function useStreamAvatar(store: LivestreamStore): string | undefined {
  const identity = useStore(
    store,
    useShallow((state) => ({
      did: state.profile?.did ?? state.livestream?.author.did,
      profileAvatar: state.profile?.avatar,
      authorAvatar: state.livestream?.author.avatar,
    })),
  );
  const dids = useMemo(
    () => (identity.did ? [identity.did] : []),
    [identity.did],
  );
  const profiles = useAvatars(dids);

  return resolveStreamAvatar({
    detailedAvatar: identity.did ? profiles[identity.did]?.avatar : undefined,
    profileAvatar: identity.profileAvatar,
    authorAvatar: identity.authorAvatar,
  });
}
