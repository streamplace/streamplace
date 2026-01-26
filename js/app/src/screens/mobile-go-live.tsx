import { useNavigation, useRoute } from "@react-navigation/native";
import { KeepAwake } from "@streamplace/components";
import Loading from "components/loading/loading";
import { Player } from "components/mobile/player";
import StreamerAgreement from "components/streamer-agreement";
import { FullscreenProvider } from "contexts/FullscreenContext";
import useStreamplaceNode from "hooks/useStreamplaceNode";
import { useEffect, useState } from "react";
import { useStore } from "store";
import { useUserProfile } from "store/hooks";

export default function MobileGoLive() {
  const userProfile = useUserProfile();
  const openLoginModal = useStore((state) => state.openLoginModal);
  const route = useRoute();
  const [agreementAccepted, setAgreementAccepted] = useState(false);
  const navigation = useNavigation();
  const nodeUrl = useStreamplaceNode();

  const isOfficialNode =
    nodeUrl ?? new URL(nodeUrl).hostname.endsWith(".stream.place");

  useEffect(() => {
    if (!userProfile) {
      openLoginModal({ name: route.name, params: route.params });
    }
  }, [userProfile, openLoginModal, route.name, route.params]);

  if (!userProfile) {
    return <Loading />;
  }

  if (!agreementAccepted && isOfficialNode) {
    return (
      <StreamerAgreement
        nodeUrl={new URL(nodeUrl.url)}
        onAccepted={() => setAgreementAccepted(true)}
        onDeclined={() =>
          navigation.canGoBack()
            ? navigation.goBack()
            : navigation.navigate("Home", { screen: "StreamList" })
        }
      />
    );
  }

  return (
    <>
      <KeepAwake />
      <FullscreenProvider>
        <Player ingest src={userProfile.did} name={userProfile.handle} />
      </FullscreenProvider>
    </>
  );
}
