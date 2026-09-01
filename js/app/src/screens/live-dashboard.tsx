import { useNavigation, useRoute } from "@react-navigation/native";
import { LivestreamProvider, PlayerProvider } from "@streamplace/components";
import BentoGrid from "components/live-dashboard/bento-grid";
import Loading from "components/loading/loading";
import StreamerAgreement from "components/streamer-agreement";
import { VideoElementProvider } from "contexts/VideoElementContext";
import { useLiveUser } from "hooks/useLiveUser";
import useStreamplaceNode from "hooks/useStreamplaceNode";
import { useCallback, useEffect, useState } from "react";
import { useStore } from "store";
import { useIsReady, useUserProfile } from "store/hooks";

export default function LiveDashboard() {
  const isReady = useIsReady();
  const userProfile = useUserProfile();
  const isLive = useLiveUser();
  const openLoginModal = useStore((state) => state.openLoginModal);
  const route = useRoute();
  const [videoElement, setVideoElement] = useState<HTMLVideoElement | null>(
    null,
  );
  const [agreementAccepted, setAgreementAccepted] = useState(false);
  const navigation = useNavigation();

  const nodeUrl = useStreamplaceNode();

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

  const isOfficialNode =
    nodeUrl ?? new URL(nodeUrl).hostname.endsWith(".stream.place");

  return (
    <>
      <LivestreamProvider src={userProfile.did}>
        <VideoElementProvider videoElement={videoElement}>
          <PlayerProvider>
            <BentoGrid isLive={isLive} videoRef={videoRef} />
          </PlayerProvider>
        </VideoElementProvider>
      </LivestreamProvider>
      {!agreementAccepted && isOfficialNode && (
        <StreamerAgreement
          nodeUrl={new URL(nodeUrl.url)}
          onAccepted={() => setAgreementAccepted(true)}
          onDeclined={() =>
            navigation.canGoBack()
              ? navigation.goBack()
              : navigation.navigate("Home", { screen: "StreamList" })
          }
        />
      )}
    </>
  );
}
