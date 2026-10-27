import { useIsFocused, useRoute } from "@react-navigation/native";
import {
  LivestreamProvider,
  PlayerProvider,
  useStreamplaceStore,
  useUrl,
} from "@streamplace/components";
import BentoGrid from "components/live-dashboard/bento-grid";
import Loading from "components/loading/loading";
import { VideoElementProvider } from "contexts/VideoElementContext";
import { ReactNode, useCallback, useEffect, useState } from "react";
import { useStore } from "store";
import { useIsReady, useUserProfile } from "store/hooks";

export default function LiveDashboard({ children }: { children?: ReactNode }) {
  const isReady = useIsReady();
  const userProfile = useUserProfile();
  const oauthSession = useStreamplaceStore((state) => state.oauthSession);
  const url = useUrl();
  const isFocused = useIsFocused();
  const openLoginModal = useStore((state) => state.openLoginModal);
  const route = useRoute();
  const ready = isReady && oauthSession !== undefined;

  useEffect(() => {
    if (isFocused && ready && !userProfile) {
      openLoginModal({ name: route.name, params: route.params });
    }
  }, [isFocused, ready, userProfile, openLoginModal, route.name, route.params]);

  if (!isFocused) return null;

  if (!ready || !userProfile || oauthSession?.did !== userProfile.did) {
    return <Loading />;
  }

  return (
    <LivestreamProvider
      key={`${url}:${userProfile.did}`}
      src={userProfile.did}
      ignoreOuterContext
    >
      {children ?? <LiveDashboardView />}
    </LivestreamProvider>
  );
}

export function LiveDashboardView() {
  const [videoElement, setVideoElement] = useState<HTMLVideoElement | null>(
    null,
  );
  const videoRef = useCallback((node: HTMLVideoElement | null) => {
    setVideoElement(node);
  }, []);

  return (
    <VideoElementProvider videoElement={videoElement}>
      <PlayerProvider>
        <BentoGrid videoRef={videoRef} />
      </PlayerProvider>
    </VideoElementProvider>
  );
}
