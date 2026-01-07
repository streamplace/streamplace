import { useRoute } from "@react-navigation/native";
import { KeepAwake } from "@streamplace/components";
import Loading from "components/loading/loading";
import { Player } from "components/mobile/player";
import { FullscreenProvider } from "contexts/FullscreenContext";
import { useEffect } from "react";
import { useStore } from "store";
import { useUserProfile } from "store/hooks";

export default function MobileGoLive() {
  const userProfile = useUserProfile();
  const openLoginModal = useStore((state) => state.openLoginModal);
  const route = useRoute();

  useEffect(() => {
    if (!userProfile) {
      openLoginModal({ name: route.name, params: route.params });
    }
  }, [userProfile, openLoginModal, route.name, route.params]);

  if (!userProfile) {
    return <Loading />;
  }
  // get player
  return (
    <>
      <KeepAwake />
      <FullscreenProvider>
        <Player ingest src={userProfile.did} name={userProfile.handle} />
      </FullscreenProvider>
    </>
  );
}
