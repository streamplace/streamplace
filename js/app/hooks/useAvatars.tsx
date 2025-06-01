import { ProfileViewDetailed } from "@atproto/api/dist/client/types/app/bsky/actor/defs";
import {
  getProfiles,
  selectCachedProfiles,
} from "features/bluesky/blueskySlice";
import { useEffect, useMemo } from "react";
import { useAppDispatch, useAppSelector } from "store/hooks";

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
