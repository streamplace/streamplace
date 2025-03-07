import { selectUserProfile } from "features/bluesky/blueskySlice";
import { selectRecentSegments } from "features/streamplace/streamplaceSlice";
import { useAppSelector } from "store/hooks";

// composite selector that tells us when the current user is live
export const useLiveUser = (): boolean => {
  const profile = useAppSelector(selectUserProfile);
  const { segments } = useAppSelector(selectRecentSegments);
  if (!profile) {
    return false;
  }
  const isLive = segments.some(
    (segment) => segment.repo && segment.repo.did === profile.did,
  );
  return isLive;
};
