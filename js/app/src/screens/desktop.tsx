import {
  LivestreamProvider,
  PlayerProvider,
  Text,
  useTheme,
  View,
} from "@streamplace/components";
import { breakpoints } from "@streamplace/components/src/lib/theme/tokens";
import { ObsCard } from "components/desktop/obs-card";
import { RenditionSettings } from "components/desktop/rendition-settings";
import StreamMonitor from "components/live-dashboard/stream-monitor";
import Loading from "components/loading/loading";
import { useLiveUser } from "hooks/useLiveUser";
import { useEffect } from "react";
import { ScrollView, useWindowDimensions } from "react-native";
import { useStore } from "store";
import { useIsReady, useUserProfile } from "store/hooks";

function DesktopInner() {
  const { theme } = useTheme();
  const isLive = useLiveUser();
  const profile = useUserProfile();
  const { width } = useWindowDimensions();
  const wide = width >= breakpoints.lg;
  const hideSidebar = useStore((x) => x.setSidebarHidden);
  const showSidebar = useStore((x) => x.setSidebarUnhidden);

  useEffect(() => {
    hideSidebar();
    return () => showSidebar();
  }, [hideSidebar, showSidebar]);

  return (
    <View
      style={{
        flex: 1,
        flexDirection: wide ? "row" : "column",
        gap: theme.spacing[4],
        padding: theme.spacing[4],
        backgroundColor: theme.colors.surface0,
      }}
    >
      <View style={wide ? { flex: 2 } : { width: "100%", aspectRatio: 16 / 9 }}>
        <StreamMonitor isLive={isLive} userProfile={profile} />
      </View>
      <ScrollView style={{ flex: 1 }}>
        <View style={{ gap: theme.spacing[6] }}>
          <Text size="xl">Streamplace Desktop</Text>
          <ObsCard />
          <RenditionSettings />
        </View>
      </ScrollView>
    </View>
  );
}

/**
 * Streamplace Desktop's home: this node's configuration for the signed-in
 * streamer, and a preview of their stream.
 */
export default function Desktop() {
  const isReady = useIsReady();
  const userProfile = useUserProfile();
  const openLoginModal = useStore((state) => state.openLoginModal);

  useEffect(() => {
    if (isReady && !userProfile) {
      openLoginModal({ name: "Desktop" });
    }
  }, [isReady, userProfile, openLoginModal]);

  if (!isReady || !userProfile) {
    return <Loading />;
  }

  return (
    <LivestreamProvider src={userProfile.did}>
      <PlayerProvider>
        <DesktopInner />
      </PlayerProvider>
    </LivestreamProvider>
  );
}
