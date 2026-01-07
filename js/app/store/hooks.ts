import { useStore } from "./index";

// Base selectors
export const useHydrated = () => useStore((state) => state.hydrated);

// Sidebar selectors
export const useIsSidebarCollapsed = () =>
  useStore((state) => state.isCollapsed);
export const useSidebarTargetWidth = () =>
  useStore((state) => state.targetWidth);
export const useIsSidebarLoaded = () => useStore((state) => state.isLoaded);
export const useIsSidebarHidden = () => useStore((state) => state.isHidden);

// Bluesky selectors
export const useOAuthSession = () => useStore((state) => state.oauthSession);
export const usePDS = () => useStore((state) => state.pds);
export const useLogin = () => useStore((state) => state.loginState);
export const useProfiles = () => useStore((state) => state.profiles);
export const useStoredKey = () => useStore((state) => state.storedKey);
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
export const useNewLivestream = () => useStore((state) => state.newLivestream);
export const useChatProfile = () => useStore((state) => state.chatProfile);
export const useCachedProfiles = () => useStore((state) => state.profileCache);

// ContentMetadata selectors
export const useContentMetadata = () =>
  useStore((state) => ({
    creating: state.creating,
    updating: state.updating,
    error: state.error,
    lastCreatedRecord: state.lastCreatedRecord,
  }));
export const useIsCreating = () => useStore((state) => state.creating);
export const useIsUpdating = () => useStore((state) => state.updating);
export const useContentMetadataError = () => useStore((state) => state.error);
export const useLastCreatedRecord = () =>
  useStore((state) => state.lastCreatedRecord);

// Streamplace selectors
export const useStreamplaceUrl = () => useStore((state) => state.url);
export const useStreamplaceInitialized = () =>
  useStore((state) => state.initialized);
export const useUserMuted = () => useStore((state) => state.userMuted);
export const useChatWarned = () => useStore((state) => state.chatWarned);
export const useMySegments = () => useStore((state) => state.mySegments);

// Platform selectors
export const useNotificationToken = () =>
  useStore((state) => state.notificationToken);
export const useNotificationDestination = () =>
  useStore((state) => state.notificationDestination);
