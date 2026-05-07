import {
  LivestreamProvider,
  PlayerProvider,
  zero,
} from "@streamplace/components";
import StreamMonitor from "components/live-dashboard/stream-monitor";
import Loading from "components/loading/loading";
import { useLiveUser } from "hooks/useLiveUser";
import { useEffect } from "react";
import { View } from "react-native";
import { useStore } from "store";
import { useIsReady, useUserProfile } from "store/hooks";

const { flex } = zero;

function PopoutStreamMonitorInner() {
  const isLive = useLiveUser();
  const profile = useUserProfile();
  const hideSidebar = useStore((x) => x.setSidebarHidden);
  const showSidebar = useStore((x) => x.setSidebarUnhidden);

  useEffect(() => {
    hideSidebar();
    return () => {
      showSidebar();
    };
  }, [showSidebar]);

  return (
    <View style={[flex.values[1]]}>
      <StreamMonitor isLive={isLive} userProfile={profile} />
    </View>
  );
}

export default function PopoutStreamMonitor() {
  const isReady = useIsReady();
  const userProfile = useUserProfile();
  const openLoginModal = useStore((state) => state.openLoginModal);

  if (!isReady) return <Loading />;
  if (!userProfile) {
    openLoginModal({ name: "PopoutStreamMonitor" });
    return <Loading />;
  }

  return (
    <LivestreamProvider src={userProfile.did}>
      <PlayerProvider>
        <PopoutStreamMonitorInner />
      </PlayerProvider>
    </LivestreamProvider>
  );
}
