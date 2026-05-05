import {
  LivestreamProvider,
  PlayerProvider,
  zero,
} from "@streamplace/components";
import LivestreamPanel from "components/live-dashboard/livestream-panel";
import Loading from "components/loading/loading";
import { useEffect } from "react";
import { View } from "react-native";
import { useStore } from "store";
import { useIsReady, useUserProfile } from "store/hooks";

const { flex, p } = zero;

export default function PopoutLivestream() {
  const isReady = useIsReady();
  const userProfile = useUserProfile();
  const openLoginModal = useStore((state) => state.openLoginModal);
  const hideSidebar = useStore((x) => x.setSidebarHidden);
  const showSidebar = useStore((x) => x.setSidebarUnhidden);

  useEffect(() => {
    hideSidebar();
    return () => {
      showSidebar();
    };
  }, [showSidebar]);

  if (!isReady) return <Loading />;
  if (!userProfile) {
    openLoginModal({ name: "PopoutLivestream" });
    return <Loading />;
  }

  return (
    <LivestreamProvider src={userProfile.did}>
      <PlayerProvider>
        <View style={[flex.values[1], p[1]]}>
          <LivestreamPanel />
        </View>
      </PlayerProvider>
    </LivestreamProvider>
  );
}
