import {
  Button,
  Input,
  MenuContainer,
  MenuGroup,
  Text,
  View,
  zero,
} from "@streamplace/components";
import { useEffect, useState } from "react";
import { useTranslation } from "react-i18next";
import { ScrollView } from "react-native";
import { useStore } from "store";
import { DEFAULT_URL } from "store/slices/streamplaceSlice";
import { SettingToggle } from "./components/setting-toggle";

export function AdvancedCategorySettings() {
  const url = useStore((state) => state.url);
  const setURL = useStore((state) => state.setURL);
  const defaultUrl = DEFAULT_URL;
  const [newUrl, setNewUrl] = useState("");
  const [overrideEnabled, setOverrideEnabled] = useState(false);
  const { t } = useTranslation("settings");

  useEffect(() => {
    setOverrideEnabled(url !== defaultUrl);
  }, [url, defaultUrl]);

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
                    placeholderTextColor="#999"
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
          </MenuContainer>
        </View>
      </View>
    </ScrollView>
  );
}
