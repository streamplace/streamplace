// Selector hooks for the combined store. Mirrors js/app/store/hooks.ts.
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

  if (authStatus === "start") {
    return false;
  } else if (authStatus === "loggedOut") {
    return true;
  }
  if (!oauthSession) {
    return false;
  }
  if (!profile) {
    return false;
  }
  return true;
};
export const useCachedProfiles = () => useStore((state) => state.profileCache);
export const useChatProfile = () => useStore((state) => state.chatProfile);

// PDS Agent (convenience — reads from bluesky slice)
export const usePDSAgent = () => useStore((state) => state.pdsAgent);
