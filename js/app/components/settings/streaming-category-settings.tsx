import {
  Button,
  ContentMetadataForm,
  Dashboard,
  MenuContainer,
  MenuGroup,
  MenuSeparator,
  Text,
  View,
  useStreamplaceStore,
  useTheme,
  zero,
} from "@streamplace/components";
import Loading from "components/loading/loading";
import { Globe, Heart, Key, Webhook } from "lucide-react-native";
import { useTranslation } from "react-i18next";
import { Platform, ScrollView } from "react-native";
import { useStore } from "store";
import { useIsReady, useUserProfile } from "store/hooks";
import { SettingsNavigationItem } from "./components/settings-navigation-item";

export function StreamingCategorySettings() {
  const { t } = useTranslation("settings");
  return (
    <ScrollView>
      <View style={[zero.layout.flex.align.center, zero.px[2], zero.py[2]]}>
        <View style={{ paddingVertical: 0, maxWidth: 500, width: "100%" }}>
          <MenuContainer>
            <MenuGroup>
              <SettingsNavigationItem
                title={t("key-management")}
                screen="KeyManagement"
                icon={Key}
              />
              <MenuSeparator />
              <SettingsNavigationItem
                title={t("recommendations-to-others")}
                screen="RecommendationsSettings"
                icon={Heart}
              />
              <MenuSeparator />
              <SettingsNavigationItem
                title={t("webhooks")}
                screen="WebhooksSettings"
                icon={Webhook}
              />
              {/* Backup is hidden until the backend is finished; its separator
                  is omitted so Webhooks/Multistream don't get a double rule. */}
              {/* <MenuSeparator />
              <SettingsNavigationItem
                title={t("backup")}
                screen="BackupSettings"
                icon={Database}
              /> */}
              <MenuSeparator />
              <SettingsNavigationItem
                title={t("multistream")}
                screen="MultistreamCategory"
                icon={Globe}
              />
            </MenuGroup>
          </MenuContainer>
        </View>
        {Platform.OS === "web" && <CreatorStreamSettings />}
      </View>
    </ScrollView>
  );
}

export function CreatorStreamingSettings() {
  const { theme } = useTheme();
  return (
    <ScrollView>
      <View
        style={{
          alignItems: "center",
          paddingHorizontal: theme.spacing[4],
          paddingTop: theme.spacing[6],
        }}
      >
        <View style={{ width: "100%", maxWidth: theme.spacing[96] * 2 }}>
          <Text size="xl">Stream Settings</Text>
          <CreatorStreamSettings />
        </View>
      </View>
    </ScrollView>
  );
}

function CreatorStreamSettings() {
  const { theme } = useTheme();
  const isReady = useIsReady();
  const profile = useUserProfile();
  const session = useStreamplaceStore((state) => state.oauthSession);
  const openLoginModal = useStore((state) => state.openLoginModal);
  if (
    !isReady ||
    session === undefined ||
    (profile && session?.did !== profile.did)
  ) {
    return <Loading />;
  }
  if (!profile) {
    return (
      <Button onPress={() => openLoginModal({ name: "StreamingCategory" })}>
        Log in to manage stream settings
      </Button>
    );
  }
  return (
    <View
      style={{
        width: "100%",
        maxWidth: theme.spacing[96] * 2,
        gap: theme.spacing[6],
        paddingVertical: theme.spacing[6],
      }}
    >
      <View style={{ gap: theme.spacing[4] }}>
        <Text size="lg">Content metadata</Text>
        <ContentMetadataForm scrollable={false} showAllSections />
      </View>
      <Dashboard.ChatAccessPanel />
    </View>
  );
}
