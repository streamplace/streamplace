import { useMemo } from "react";
import { useAuthor } from "./useAuthor";
import { useAvatars } from "./useAvatars";

// Surfaces the author's avatar URL for whatever the player is currently
// showing, switching on the player's mode via useAuthor. In both modes we
// prefer the hydrated ProfileViewDetailed avatar (fetched through the profile
// cache) and fall back to whatever the basic profile/author view carried.
// Returns undefined when no avatar is known yet.
export function useAvatar(): string | undefined {
  const author = useAuthor();
  const did = author?.did;
  // memoized so useAvatars' effect doesn't refire on every render
  const dids = useMemo(() => (did ? [did] : []), [did]);
  const detailed = useAvatars(dids);
  if (!did) return undefined;
  return detailed[did]?.avatar ?? author?.avatar;
}
