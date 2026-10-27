import {
  Dashboard,
  LivestreamProvider,
  PlayerProvider,
  zero,
} from "@streamplace/components";
import Loading from "components/loading/loading";
import { useLiveDashboardStatus } from "hooks/useLiveDashboardStatus";
import { useEffect } from "react";
import { View } from "react-native";
import { useStore } from "store";
import { useIsReady, useUserProfile } from "store/hooks";

const { flex, layout, p } = zero;

function StreamHealth() {
  const status = useLiveDashboardStatus();
  return (
    <Dashboard.InformationWidget
      readingsStale={status.readingsStale}
      connectionQuality={status.connectionQuality}
    />
  );
}

export default function PopoutInfoWidget() {
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
    openLoginModal({ name: "PopoutInfoWidget" });
    return <Loading />;
  }

  return (
    <LivestreamProvider src={userProfile.did}>
      <PlayerProvider>
        <View
          style={[
            flex.values[1],
            layout.flex.alignCenter,
            layout.flex.justifyCenter,
            p[1],
          ]}
        >
          <StreamHealth />
        </View>
      </PlayerProvider>
    </LivestreamProvider>
  );
}
