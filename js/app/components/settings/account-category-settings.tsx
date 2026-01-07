import { useNavigation } from "@react-navigation/native";
import {
  Button,
  MenuContainer,
  MenuGroup,
  MenuSeparator,
  Text,
  useTheme,
  useTranslation,
  View,
  zero,
} from "@streamplace/components";
import { useNameColorPicker } from "components/name-color-picker/name-color-picker";
import { Edit3, LogOut, Palette, X } from "lucide-react-native";
import { Image, ScrollView } from "react-native";
import { useStore } from "store";
import { useChatProfile, useUserProfile } from "store/hooks";
import {
  SettingsExternalItem,
  SettingsRowItem,
} from "./components/settings-navigation-item";

export function AccountCategorySettings() {
  const { theme, zero: z } = useTheme();
  const { t } = useTranslation("settings");
  const { t: tn } = useTranslation();
  const logout = useStore((state) => state.logout);
  const chatProfile = useChatProfile();
  const userProfile = useUserProfile();
  const { currentColor, openModal, modal } = useNameColorPicker();

  const navigation = useNavigation();

  if (!userProfile) {
    // do a "log in to access this page, or go back to settings"
    return (
      <View
        style={[
          zero.layout.flex.align.center,
          zero.px[2],
          zero.gap.all[4],
          zero.mt[8],
        ]}
      >
        <View style={[zero.p[4], zero.r.full, z.bg.destructive]}>
          <X size={48} color={theme.colors.destructiveForeground} />
        </View>
        <Text size="lg">{tn("please-log-in-to-access-this-page")}</Text>
        <View>
          <Button
            width="min"
            variant="secondary"
            onPress={() =>
              navigation.canGoBack()
                ? navigation.goBack()
                : navigation.navigate("Settings", { screen: "MainSettings" })
            }
          >
            {tn("go-back")}
          </Button>
        </View>
      </View>
    );
  }

  let rgb =
    chatProfile.profile?.color &&
    `rgb(${chatProfile.profile?.color?.red},${chatProfile.profile?.color?.green},${chatProfile.profile?.color?.blue})`;

  return (
    <ScrollView>
      <View style={[zero.layout.flex.align.center, zero.px[2]]}>
        <View style={{ paddingTop: 24, maxWidth: 500, width: "100%" }}>
          <View style={[zero.layout.flex.align.center, zero.pt[4], zero.pb[2]]}>
            {userProfile.avatar && (
              <Image
                source={{ uri: userProfile.avatar }}
                style={{
                  width: 80,
                  height: 80,
                  borderRadius: 40,
                  marginBottom: 12,
                }}
              />
            )}
            <Text size="3xl" style={{ textAlign: "center" }}>
              {(() => {
                const greeting = t("account-greeting", {
                  handle: userProfile.handle,
                });
                const handle = `@${userProfile.handle}`;
                const parts = greeting.split(handle);
                return (
                  <>
                    {parts[0]}
                    <Text size="3xl" style={{ color: rgb || "#bd6e86" }}>
                      {handle}
                    </Text>
                    {parts[1]}
                  </>
                );
              })()}
            </Text>
          </View>

          <MenuContainer>
            <MenuGroup>
              <SettingsExternalItem
                LeftIcon={Edit3}
                title={t("edit-profile-bluesky")}
                link={`https://bsky.app/profile/${userProfile.handle}`}
              />
            </MenuGroup>

            <MenuGroup>
              <SettingsRowItem onPress={openModal}>
                <View
                  style={{
                    flexDirection: "row",
                    alignItems: "center",
                    gap: 12,
                  }}
                >
                  <Palette size={20} color={currentColor} />
                  <Text size="lg">{t("change-name-color")}</Text>
                </View>
              </SettingsRowItem>
              <MenuSeparator />
              <SettingsRowItem
                onPress={() => {
                  logout();
                  // wait a bit to debounce
                  setTimeout(() => {
                    navigation.navigate("Settings", { screen: "MainSettings" });
                  }, 100);
                }}
              >
                <View
                  style={{
                    flexDirection: "row",
                    alignItems: "center",
                    gap: 12,
                  }}
                >
                  <LogOut size={20} color={theme.colors.textMuted} />
                  <Text size="lg">{t("log-out")}</Text>
                </View>
              </SettingsRowItem>
            </MenuGroup>
          </MenuContainer>
        </View>
      </View>
      {modal}
    </ScrollView>
  );
}
