import {
  MenuContainer,
  MenuGroup,
  useBetaStatus,
  useToast,
  View,
  zero,
} from "@streamplace/components";
import { usePDSAgent } from "@streamplace/components/src/streamplace-store/xrpc";
import { useEffect, useState } from "react";
import { useTranslation } from "react-i18next";
import { ScrollView } from "react-native";
import { useStore } from "store";
import { useIsReady, useServerSettings, useStreamplaceUrl } from "store/hooks";
import { place } from "streamplace";
import { SettingToggle } from "./components/setting-toggle";

// Automatic VOD publishing is a preference the node keeps itself
// (place.stream.server.getPreferences), not part of the settings record.
function AutoPublishVodsToggle({ host }: { host: string }) {
  const { t } = useTranslation(["settings", "common"]);
  const toast = useToast();
  const agent = usePDSAgent();
  const [enabled, setEnabled] = useState<boolean | null>(null);

  useEffect(() => {
    if (!agent) return;
    agent.client
      .call(place.stream.server.getPreferences)
      .then((res) => setEnabled(res.preferences.autoPublishVods))
      .catch((err) => console.error("Failed to load preferences:", err));
  }, [agent]);

  if (enabled === null) {
    return null;
  }

  const handleChange = async (value: boolean) => {
    if (!agent) return;
    setEnabled(value);
    try {
      const res = await agent.client.call(place.stream.server.putPreferences, {
        autoPublishVods: value,
      });
      setEnabled(res.preferences.autoPublishVods);
    } catch (err) {
      console.error("Failed to update preferences:", err);
      setEnabled(!value);
      toast.show(
        t("common:error"),
        err instanceof Error && err.message
          ? err.message
          : t("auto-publish-vods-update-failed"),
        { variant: "error" },
      );
    }
  };

  return (
    <SettingToggle
      title={t("auto-publish-vods-title")}
      description={t("auto-publish-vods-description", { host })}
      value={enabled}
      onValueChange={handleChange}
      testID="settings-auto-publish-vods"
    />
  );
}

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
  // Defaults on (unlike debugRecording): only an explicit `false` turns it off.
  const livestreamRecordingOn = serverSettings?.livestreamRecording !== false;
  // The livestream-recording and automatic VOD publishing toggles are only
  // meaningful for accounts in the VOD beta — the node won't record anyone
  // else regardless of these flags — so we only surface them to them.
  const { status: vodBetaStatus } = useBetaStatus("vod");

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
                  createServerSettingsRecord({ debugRecording: value });
                }}
              />
              {vodBetaStatus === "granted" && (
                <SettingToggle
                  title={t("livestream-recording-title", { host: u.host })}
                  description={t("livestream-recording-description")}
                  value={livestreamRecordingOn}
                  onValueChange={(value) => {
                    createServerSettingsRecord({ livestreamRecording: value });
                  }}
                />
              )}
              {vodBetaStatus === "granted" && (
                <AutoPublishVodsToggle host={u.host} />
              )}
            </MenuGroup>
          </MenuContainer>
        </View>
      </View>
    </ScrollView>
  );
}
