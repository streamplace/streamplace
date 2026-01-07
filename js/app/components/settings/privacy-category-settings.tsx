import { MenuContainer, MenuGroup, View, zero } from "@streamplace/components";
import { useEffect } from "react";
import { useTranslation } from "react-i18next";
import { ScrollView } from "react-native";
import { useStore } from "store";
import { useIsReady, useServerSettings, useStreamplaceUrl } from "store/hooks";
import { SettingToggle } from "./components/setting-toggle";

export function PrivacyCategorySettings() {
  const { t } = useTranslation("settings");
  const isReady = useIsReady();
  const serverSettings = useServerSettings();
  const url = useStreamplaceUrl();
  const getServerSettingsFromPDS = useStore(
    (state) => state.getServerSettingsFromPDS,
  );
  const createServerSettingsRecord = useStore(
    (state) => state.createServerSettingsRecord,
  );
  const debugRecordingOn = serverSettings?.debugRecording === true;

  useEffect(() => {
    if (isReady) {
      getServerSettingsFromPDS();
    }
  }, [isReady]);

  const u = new URL(url);

  return (
    <ScrollView>
      <View style={[zero.layout.flex.align.center, zero.px[2], zero.py[2]]}>
        <View style={{ maxWidth: 500, width: "100%" }}>
          <MenuContainer>
            <MenuGroup>
              <SettingToggle
                title={t("debug-recording-title", { host: u.host })}
                description={t("debug-recording-description")}
                value={debugRecordingOn}
                onValueChange={(value) => {
                  if (value === true) {
                    createServerSettingsRecord(true);
                  } else {
                    createServerSettingsRecord(false);
                  }
                }}
              />
            </MenuGroup>
          </MenuContainer>
        </View>
      </View>
    </ScrollView>
  );
}
