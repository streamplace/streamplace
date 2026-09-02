import {
  Button,
  Input,
  Loader,
  MenuContainer,
  MenuGroup,
  Text,
  useFetchBranding,
  useTheme,
  View,
  zero,
} from "@streamplace/components";
import { Check } from "lucide-react-native";
import { useEffect, useState } from "react";
import { useTranslation } from "react-i18next";
import { Platform, ScrollView } from "react-native";
import { useStore } from "store";
import { DEFAULT_URL } from "store/slices/streamplaceSlice";
import { SettingToggle } from "./components/setting-toggle";
import { SettingsRowItem } from "./components/settings-navigation-item";

const WEB_BETA_COOKIE = "sp_web_beta";
const ONE_YEAR_SECONDS = 60 * 60 * 24 * 365;

function isWebBetaEnabledCookie(): boolean {
  if (typeof document === "undefined") return false;
  const cookies = document.cookie.split(";");
  for (const raw of cookies) {
    const [name, ...rest] = raw.trim().split("=");
    if (name === WEB_BETA_COOKIE) {
      return rest.join("=") === "1";
    }
  }
  return false;
}

function setWebBetaCookie(enabled: boolean) {
  if (typeof document === "undefined") return;
  if (enabled) {
    document.cookie = `${WEB_BETA_COOKIE}=1; path=/; max-age=${ONE_YEAR_SECONDS}; SameSite=Lax`;
  } else {
    document.cookie = `${WEB_BETA_COOKIE}=; path=/; max-age=0; SameSite=Lax`;
  }
}

type Status = "ready" | "active" | "done";

export function AdvancedCategorySettings() {
  const { theme } = useTheme();
  const url = useStore((state) => state.url);
  const setURL = useStore((state) => state.setURL);
  const defaultUrl = DEFAULT_URL;
  const [newUrl, setNewUrl] = useState("");
  const [overrideEnabled, setOverrideEnabled] = useState(false);
  const [webBeta, setWebBeta] = useState(false);
  const { t } = useTranslation("settings");

  const fetchBranding = useFetchBranding();

  const [refreshBranding, setRefreshBranding] = useState<Status>("ready");

  useEffect(() => {
    setOverrideEnabled(url !== defaultUrl);
  }, [url, defaultUrl]);

  useEffect(() => {
    setWebBeta(isWebBetaEnabledCookie());
  }, []);

  const onSubmitUrl = () => {
    if (newUrl) {
      let trimmedUrl = newUrl.endsWith("/") ? newUrl.slice(0, -1) : newUrl;
      setURL(trimmedUrl);
      setNewUrl("");
    }
  };

  const handleToggleOverride = (enabled: boolean) => {
    setOverrideEnabled(enabled);
    if (!enabled) {
      setURL(defaultUrl);
    }
  };

  // Toggling web-beta writes a cookie and reloads. Web context only —
  // the cookie lives in the browser, and the new web frontend is what
  // we serve to opted-in users. Native mobile users have their own app
  // and don't visit the web frontend.
  const handleWebBetaToggle = (enabled: boolean) => {
    setWebBetaCookie(enabled);
    if (typeof window !== "undefined") {
      window.location.reload();
    }
  };

  return (
    <ScrollView>
      <View style={[zero.layout.flex.align.center, zero.px[2], zero.py[2]]}>
        <View style={{ maxWidth: 500, width: "100%" }}>
          <MenuContainer>
            <MenuGroup>
              <SettingToggle
                title={t("use-custom-node")}
                description={t("default-url", { url: defaultUrl })}
                value={overrideEnabled}
                onValueChange={handleToggleOverride}
              />
            </MenuGroup>

            {Platform.OS === "web" && (
              <MenuGroup>
                <SettingToggle
                  title={t("try-new-web")}
                  description={t("try-new-web-description")}
                  value={webBeta}
                  onValueChange={handleWebBetaToggle}
                />
              </MenuGroup>
            )}

            {overrideEnabled && (
              <View
                style={[
                  {
                    opacity: overrideEnabled ? 1 : 0,
                    height: overrideEnabled ? "auto" : 0,
                  },
                  zero.gap.all[2],
                  zero.layout.flex.align.center,
                  zero.layout.flex.row,
                  { marginTop: 12 },
                ]}
              >
                <View style={{ flex: 1 }}>
                  <Input
                    value={newUrl}
                    containerStyle={[
                      { flex: 1, flexGrow: 1, width: "100%" },
                      zero.flex.grow[1],
                    ]}
                    variant="default"
                    numberOfLines={1}
                    multiline={false}
                    placeholder={
                      url != defaultUrl ? url : t("enter-custom-node-url")
                    }
                    placeholderTextColor={theme.colors.textMuted}
                    onChangeText={setNewUrl}
                    onSubmitEditing={onSubmitUrl}
                    textContentType="URL"
                    autoCapitalize="none"
                    autoCorrect={false}
                    keyboardType="url"
                  />
                </View>
                <Button
                  size="md"
                  width="min"
                  variant="secondary"
                  onPress={onSubmitUrl}
                  style={{ paddingVertical: 10 }}
                >
                  <Text size="lg">{t("save-button")}</Text>
                </Button>
              </View>
            )}
            <MenuGroup>
              <SettingsRowItem
                onPress={() => {
                  setRefreshBranding("active");
                  fetchBranding({ force: true }).then(() => {
                    setRefreshBranding("done");
                    // set back to ready after a short delay
                    setTimeout(() => {
                      setRefreshBranding("ready");
                    }, 2500);
                  });
                }}
              >
                <View style={{ flex: 1 }}>
                  <Text size="lg">{t("refresh-branding")}</Text>
                </View>
                <View>
                  {refreshBranding === "active" && <Loader />}
                  {refreshBranding === "done" && (
                    <Check size={20} color={theme.colors.success} />
                  )}
                </View>
              </SettingsRowItem>
            </MenuGroup>
          </MenuContainer>
        </View>
      </View>
    </ScrollView>
  );
}
