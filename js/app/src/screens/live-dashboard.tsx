import { useRoute } from "@react-navigation/native";
import {
  LivestreamProvider,
  PlayerProvider,
  useLivestreamStore,
} from "@streamplace/components";
import BentoGrid from "components/live-dashboard/bento-grid";
import Loading from "components/loading/loading";
import { VideoElementProvider } from "contexts/VideoElementContext";
import { useCallback, useEffect, useState } from "react";
import { useStore } from "store";
import { useIsReady, useUserProfile } from "store/hooks";

export default function LiveDashboard() {
  const isReady = useIsReady();
  const userProfile = useUserProfile();
  const openLoginModal = useStore((state) => state.openLoginModal);
  const route = useRoute();
  const [videoElement, setVideoElement] = useState<HTMLVideoElement | null>(
    null,
  );

  const videoRef = useCallback((node: HTMLVideoElement | null) => {
    if (node !== null) {
      setVideoElement(node);
    }
  }, []);

  useEffect(() => {
    if (isReady && !userProfile) {
      openLoginModal({ name: route.name, params: route.params });
    }
  }, [isReady, userProfile, openLoginModal, route.name, route.params]);

  if (!isReady) {
    return <Loading />;
  }

  if (!userProfile) {
    return <Loading />;
  }

  return (
    <LivestreamProvider src={userProfile.did}>
      <VideoElementProvider videoElement={videoElement}>
        <PlayerProvider>
          <LiveDashboardInner videoRef={videoRef} />
        </PlayerProvider>
      </VideoElementProvider>
    </LivestreamProvider>
  );
}

export function LiveDashboardInner({
  videoRef,
}: {
  videoRef: (node: HTMLVideoElement | null) => void;
}) {
  const originUpdatedAt = useLivestreamStore((state) => state.originUpdatedAt);
  const isLive =
    originUpdatedAt !== null && originUpdatedAt > Date.now() - 1000 * 60 * 5;
  return <BentoGrid isLive={isLive} videoRef={videoRef} />;
}
