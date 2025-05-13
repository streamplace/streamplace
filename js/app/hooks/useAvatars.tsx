import { useEffect, useMemo } from "react";
import { ProfileViewDetailed } from "@atproto/api/dist/client/types/app/bsky/actor/defs";
import { useAppSelector, useAppDispatch } from "store/hooks";
import {
  selectCachedProfiles,
  getProfiles,
} from "features/bluesky/blueskySlice";

// Hack: Easy way to cache and get avatars
export default function useAvatars(dids: string[]) {
  const dispatch = useAppDispatch();
  const profiles: Record<string, ProfileViewDetailed> =
    useAppSelector(selectCachedProfiles);

  const missingDids = useMemo(
    () => dids.filter((did) => !(did in profiles)),
    [dids, profiles],
  );

  useEffect(() => {
    if (missingDids.length > 0) {
      console.log("Fetching profiles for DIDs:", missingDids);
      dispatch(getProfiles(missingDids)).then((e) => console.log("ok", e));
    }
  }, [missingDids, dispatch]);

  return profiles;
}
