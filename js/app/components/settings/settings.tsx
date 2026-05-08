import {
  MenuContainer,
  MenuGroup,
  MenuInfo,
  MenuSeparator,
  Text,
  useDanmuUnlocked,
  useDID,
  useStreamplaceStore,
  useTranslation,
  View,
  zero,
} from "@streamplace/components";
import {
  SettingsNavigationItem,
  SettingsRowItem,
} from "components/settings/components/settings-navigation-item";
import { ImageBackground } from "expo-image";
import {
  Award,
  Brush,
  FileText,
  Globe,
  Info,
  Lock,
  LogIn,
  Shield,
  User2,
  Video,
} from "lucide-react-native";
import { ScrollView } from "react-native";

import { LiquidGlassView } from "@callstack/liquid-glass";
import Mu from "components/mobile/desktop-ui/mu";
import { useStore } from "store";
import { useUserProfile } from "store/hooks";
import pkg from "../../package.json";

export function Settings() {
  const loggedIn = useStore((state) => state.authStatus === "loggedIn");
  const userProfile = useUserProfile();
  const danmuUnlocked = useDanmuUnlocked();
  const openLoginModal = useStore((state) => state.openLoginModal);

  const adminDids = useStreamplaceStore((state) => state.adminDIDs);
  const did = useDID();

  // Determine if the user is an admin
  const isAdmin = did && adminDids && adminDids.includes(did) ? true : false;

  const { t } = useTranslation("settings");

  return (
    <ScrollView>
      <View style={[zero.layout.flex.align.center, zero.px[2], zero.py[2]]}>
        <View style={{ maxWidth: 500, width: "100%" }}>
          <MenuContainer>
            <MenuGroup>
              {loggedIn && userProfile ? (
                <SettingsRowItem>
                  <View
                    style={[
                      zero.layout.flex.row,
                      zero.layout.flex.align.center,
                      zero.gap.all[4],
                      zero.py[2],
                    ]}
                  >
                    <LiquidGlassView
                      interactive
                      style={{
                        width: 48,
                        height: 48,
                        borderRadius: 24,
                      }}
                    >
                      <ImageBackground
                        source={{ uri: userProfile.avatar }}
                        style={{
                          width: 48,
                          height: 48,
                          borderRadius: 24,
                          overflow: "hidden",
                        }}
                      />
                    </LiquidGlassView>
                    <View style={{ flex: 1 }}>
                      <Text size="2xl" leading="tight">
                        @{userProfile.handle}
                      </Text>
                    </View>
                  </View>
                </SettingsRowItem>
              ) : (
                <SettingsRowItem onPress={() => openLoginModal()}>
                  <View
                    style={[
                      zero.layout.flex.row,
                      zero.layout.flex.align.center,
                      zero.gap.all[4],
                      zero.py[2],
                    ]}
                  >
                    <View
                      style={{
                        width: 48,
                        height: 48,
                        borderRadius: 24,
                        backgroundColor: "#333",
                        alignItems: "center",
                        justifyContent: "center",
                      }}
                    >
                      <LogIn size={24} color="#999" />
                    </View>
                    <View style={{ flex: 1 }}>
                      <Text size="xl" style={{ fontWeight: "600" }}>
                        {t("sign-in")}
                      </Text>
                    </View>
                  </View>
                </SettingsRowItem>
              )}
            </MenuGroup>

            {loggedIn && (
              <MenuGroup>
                <SettingsNavigationItem
                  title={t("account")}
                  screen="AccountCategory"
                  icon={User2}
                />
                <MenuSeparator />
                <SettingsNavigationItem
                  title={t("bio")}
                  screen="BioSettings"
                  icon={FileText}
                />
                <MenuSeparator />
                <SettingsNavigationItem
                  title={t("streaming")}
                  screen="StreamingCategory"
                  icon={Video}
                />
                <MenuSeparator />
                <SettingsNavigationItem
                  title={t("privacy-security")}
                  screen="PrivacyCategory"
                  icon={Shield}
                />
              </MenuGroup>
            )}
            {danmuUnlocked && (
              <MenuGroup>
                <SettingsNavigationItem
                  title={t("danmu")}
                  screen="DanmuCategory"
                  icon={Mu as any}
                />
                <MenuSeparator />
                <SettingsNavigationItem
                  title={t("issue-badges")}
                  screen="BadgeIssuer"
                  icon={Award}
                />
              </MenuGroup>
            )}
            {isAdmin && (
              <MenuGroup>
                <SettingsNavigationItem
                  title={t("branding")}
                  screen="BrandingAdmin"
                  icon={Brush}
                />
              </MenuGroup>
            )}
            <MenuGroup>
              <SettingsNavigationItem
                title={t("languages")}
                screen="LanguagesCategory"
                icon={Globe}
              />
              <MenuSeparator />
              <SettingsNavigationItem
                title={t("advanced")}
                screen="AdvancedCategory"
                icon={Lock}
              />
              <MenuSeparator />
              <SettingsNavigationItem
                title={t("about")}
                screen="AboutCategory"
                icon={Info}
              />
            </MenuGroup>
            <MenuInfo
              description={t("app-version", { version: pkg.version })}
            />
          </MenuContainer>
        </View>
      </View>
    </ScrollView>
  );
}
