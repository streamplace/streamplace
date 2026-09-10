import { useRoute } from "@react-navigation/native";
import { LivestreamProvider, PlayerProvider } from "@streamplace/components";
import {
  NodeSessionRequired,
  useBearerSession,
} from "components/access/node-session-required";
import BentoGrid from "components/live-dashboard/bento-grid";
import Loading from "components/loading/loading";
import { VideoElementProvider } from "contexts/VideoElementContext";
import { useLiveUser } from "hooks/useLiveUser";
import { useCallback, useEffect, useState } from "react";
import { useStore } from "store";
import { useIsReady, useUserProfile } from "store/hooks";

export default function LiveDashboard() {
  const isReady = useIsReady();
  const userProfile = useUserProfile();
  const isLive = useLiveUser();
  const openLoginModal = useStore((state) => state.openLoginModal);
  const bearerSession = useBearerSession();
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

  // A session inherited from the network's app can't start a stream: the
  // node authorizes that itself and only knows its own OAuth sessions.
  if (bearerSession) {
    return (
      <NodeSessionRequired
        returnRoute={{ name: route.name, params: route.params }}
      />
    );
  }

  return (
    <LivestreamProvider src={userProfile.did}>
      <VideoElementProvider videoElement={videoElement}>
        <PlayerProvider>
          <BentoGrid isLive={isLive} videoRef={videoRef} />
        </PlayerProvider>
      </VideoElementProvider>
    </LivestreamProvider>
  );
}
