import {
  MenuContainer,
  MenuGroup,
  MenuInfo,
  MenuSeparator,
  Text,
  useDanmuUnlocked,
  useTranslation,
  View,
  zero,
} from "@streamplace/components";
import {
  SettingsNavigationItem,
  SettingsRowItem,
} from "components/settings/components/settings-navigation-item";
import {
  Globe,
  Info,
  Lock,
  LogIn,
  Shield,
  User2,
  Video,
} from "lucide-react-native";
import { ImageBackground, ScrollView } from "react-native";

import { useNavigationState } from "@react-navigation/native";
import Mu from "components/mobile/desktop-ui/mu";
import { useStore } from "store";
import { useUserProfile } from "store/hooks";
import pkg from "../../package.json";

export function Settings() {
  const loggedIn = useStore((state) => state.authStatus === "loggedIn");
  const userProfile = useUserProfile();
  const danmuUnlocked = useDanmuUnlocked();
  const openLoginModal = useStore((state) => state.openLoginModal);

  // get the deepest active route for nested navigators
  const currentRoute = useNavigationState((state) => {
    let route: any = state.routes[state.index];
    while (route.state?.index !== undefined) {
      route = route.state.routes[route.state.index];
    }
    return { name: route.name, params: route.params };
  });

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
                    <ImageBackground
                      source={{ uri: userProfile.avatar }}
                      style={{
                        width: 48,
                        height: 48,
                        borderRadius: 24,
                        overflow: "hidden",
                      }}
                    />
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
