// Selector hooks for the combined store.
import { useStore } from "./index";

// Streamplace
export const useStreamplaceUrl = () => useStore((state) => state.url);
export const useStreamplaceInitialized = () =>
  useStore((state) => state.initialized);

// Bluesky / OAuth
export const useOAuthSession = () => useStore((state) => state.oauthSession);
export const useProfiles = () => useStore((state) => state.profiles);
export const useKeyRecords = () =>
  useStore((state) => state.streamKeysResponse);
export const useServerSettings = () =>
  useStore((state) => state.serverSettings);
export const useUserProfile = () => {
  const oauthSession = useOAuthSession();
  const profiles = useProfiles();
  const did = oauthSession?.did;
  if (!did) return null;
  return profiles[did];
};
export const useIsReady = () => {
  const authStatus = useStore((state) => state.authStatus);
  const oauthSession = useOAuthSession();
  const profile = useUserProfile();
  const profileError = useStore((state) => state.profileError);

  if (authStatus === "start") {
    return false;
  } else if (authStatus === "loggedOut") {
    return true;
  }
  if (!oauthSession) {
    return false;
  }
  // Profile fetch failed; session is valid, just the profile didn't
  // load. Don't block the app on a retry loop; the user can see the
  // app and try again.
  if (profileError) {
    return true;
  }
  if (!profile) {
    return false;
  }
  return true;
};
export const useCachedProfiles = () => useStore((state) => state.profileCache);
export const useChatProfile = () => useStore((state) => state.chatProfile);

// PDS Agent (convenience; reads from bluesky slice)
export const usePDSAgent = () => useStore((state) => state.pdsAgent);

// Branding
export const useSiteTitle = () => {
  const asset = useStore((state) => state.branding?.["siteTitle"]);
  return asset?.data || "Streamplace";
};
