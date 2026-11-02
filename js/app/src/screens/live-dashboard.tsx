import { useIsFocused, useRoute } from "@react-navigation/native";
import {
  Button,
  LivestreamProvider,
  PlayerProvider,
  Text,
  useStreamplaceStore,
  useTheme,
  useUrl,
} from "@streamplace/components";
import CreatorDashboard from "components/live-dashboard/creator-dashboard";
import Loading from "components/loading/loading";
import { VideoElementProvider } from "contexts/VideoElementContext";
import { ReactNode, useCallback, useEffect, useState } from "react";
import { useTranslation } from "react-i18next";
import { Platform, View } from "react-native";
import { useStore } from "store";
import { useIsReady, useUserProfile } from "store/hooks";

export default function LiveDashboard({ children }: { children?: ReactNode }) {
  const isReady = useIsReady();
  const userProfile = useUserProfile();
  const oauthSession = useStreamplaceStore((state) => state.oauthSession);
  const url = useUrl();
  const isFocused = useIsFocused();
  const openLoginModal = useStore((state) => state.openLoginModal);
  const route = useRoute();
  const ready = isReady && oauthSession !== undefined;

  useEffect(() => {
    if (isFocused && ready && !userProfile) {
      openLoginModal({ name: route.name, params: route.params });
    }
  }, [isFocused, ready, userProfile, openLoginModal, route.name, route.params]);

  if (!isFocused) return null;

  if (ready && !userProfile && Platform.OS === "web") {
    return (
      <DashboardLoginPrompt
        onLogin={() =>
          openLoginModal({ name: route.name, params: route.params })
        }
      />
    );
  }

  if (!ready || !userProfile || oauthSession?.did !== userProfile.did) {
    return <Loading />;
  }

  return (
    <LivestreamProvider
      key={`${url}:${userProfile.did}`}
      src={userProfile.did}
      ignoreOuterContext
    >
      {children ?? <LiveDashboardView />}
    </LivestreamProvider>
  );
}

function DashboardLoginPrompt({ onLogin }: { onLogin: () => void }) {
  const { theme } = useTheme();
  const { t } = useTranslation("common");
  return (
    <View
      style={{
        flex: 1,
        alignItems: "center",
        justifyContent: "center",
        gap: theme.spacing[4],
        padding: theme.spacing[4],
        backgroundColor: theme.colors.surface0,
      }}
    >
      <Text>
        {t("creator-login-required", {
          defaultValue: "Log in to access the creator dashboard.",
        })}
      </Text>
      <Button width="min" onPress={onLogin}>
        {t("log-in")}
      </Button>
    </View>
  );
}

export function LiveDashboardView() {
  const [videoElement, setVideoElement] = useState<HTMLVideoElement | null>(
    null,
  );
  const videoRef = useCallback((node: HTMLVideoElement | null) => {
    setVideoElement(node);
  }, []);

  return (
    <VideoElementProvider videoElement={videoElement}>
      <PlayerProvider>
        <CreatorDashboard videoRef={videoRef} />
      </PlayerProvider>
    </VideoElementProvider>
  );
}
