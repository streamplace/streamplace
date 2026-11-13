import {
  Button,
  LivestreamProvider,
  PlayerProvider,
  Text,
  useTheme,
  View,
} from "@streamplace/components";
import { breakpoints } from "@streamplace/components/src/lib/theme/tokens";
import { ForwardingCard } from "components/desktop/forwarding-card";
import { LogsCard } from "components/desktop/logs-card";
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
  const logout = useStore((state) => state.logout);
  const { width } = useWindowDimensions();
  const wide = width >= breakpoints.lg;

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
          <View
            style={{
              flexDirection: "row",
              alignItems: "center",
              justifyContent: "space-between",
              gap: theme.spacing[3],
            }}
          >
            <Text color="muted">Signed in as @{profile?.handle}</Text>
            <Button
              variant="secondary"
              testID="desktop-logout"
              onPress={() => logout()}
            >
              Log out
            </Button>
          </View>
          <ForwardingCard />
          <ObsCard />
          <RenditionSettings />
          <LogsCard />
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
  const { theme } = useTheme();
  const isReady = useIsReady();
  const userProfile = useUserProfile();
  const openLoginModal = useStore((state) => state.openLoginModal);
  const hideSidebar = useStore((x) => x.setSidebarHidden);
  const showSidebar = useStore((x) => x.setSidebarUnhidden);

  // Desktop is the only screen, signed in or not; nothing to navigate to.
  useEffect(() => {
    hideSidebar();
    return () => showSidebar();
  }, [hideSidebar, showSidebar]);

  useEffect(() => {
    if (isReady && !userProfile) {
      openLoginModal({ name: "Desktop" });
    }
  }, [isReady, userProfile, openLoginModal]);

  if (!isReady) {
    return <Loading />;
  }
  // The login modal can be dismissed; keep a way back to it.
  if (!userProfile) {
    return (
      <View
        style={{
          flex: 1,
          alignItems: "center",
          justifyContent: "center",
          gap: theme.spacing[4],
          backgroundColor: theme.colors.surface0,
        }}
      >
        <Text size="lg">Log in to start streaming.</Text>
        <Button
          testID="desktop-login"
          onPress={() => openLoginModal({ name: "Desktop" })}
        >
          Log in
        </Button>
      </View>
    );
  }

  return (
    <LivestreamProvider src={userProfile.did}>
      <PlayerProvider>
        <DesktopInner />
      </PlayerProvider>
    </LivestreamProvider>
  );
}
