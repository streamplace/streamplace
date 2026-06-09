import { Card, CardRow } from "@/components/ui/card";
import { Switch } from "@/components/ui/switch";
import { createFileRoute } from "@tanstack/react-router";
import { useEffect } from "react";
import { useTranslation } from "react-i18next";
import { useStore } from "../../lib/store";
import {
  useIsReady,
  useServerSettings,
  useStreamplaceUrl,
} from "../../lib/store/hooks";

export const Route = createFileRoute("/settings/privacy")({
  component: PrivacySettings,
});

function PrivacySettings() {
  const { t } = useTranslation("settings");
  const isReady = useIsReady();
  const serverSettings = useServerSettings();
  const url = useStreamplaceUrl();
  const getServerSettingsFromPDS = useStore((s) => s.getServerSettingsFromPDS);
  const createServerSettingsRecord = useStore(
    (s) => s.createServerSettingsRecord,
  );

  useEffect(() => {
    if (isReady) getServerSettingsFromPDS();
  }, [isReady]);

  const debugRecordingOn = serverSettings?.debugRecording === true;
  const u = new URL(url);

  return (
    <div className="space-y-6">
      <h1 className="text-xl font-semibold font-display">{t("privacy-security")}</h1>

      <Card>
        <CardRow>
          <div className="flex items-center justify-between">
            <div className="pr-4">
              <div className="text-sm font-medium">
                {t("debug-recording-title", { host: u.host })}
              </div>
              <div className="text-xs text-[var(--color-fg-muted)] mt-0.5">
                {t("debug-recording-description")}
              </div>
            </div>
            <Switch
              checked={debugRecordingOn}
              onCheckedChange={() =>
                createServerSettingsRecord(!debugRecordingOn)
              }
            />
          </div>
        </CardRow>
      </Card>
    </div>
  );
}
