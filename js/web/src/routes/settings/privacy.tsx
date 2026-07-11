import { Card, CardRow } from "@/components/ui/card";
import { Switch } from "@/components/ui/switch";
import { useSession } from "@/lib/session";
import { createFileRoute } from "@tanstack/react-router";
import { useEffect, useState } from "react";
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
  const { state: sessionState } = useSession();
  const getServerSettingsFromPDS = useStore((s) => s.getServerSettingsFromPDS);
  const createServerSettingsRecord = useStore(
    (s) => s.createServerSettingsRecord,
  );
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const isAuthenticated = sessionState.status === "authenticated";

  useEffect(() => {
    if (isReady && isAuthenticated) getServerSettingsFromPDS();
  }, [isReady, isAuthenticated]);

  const debugRecordingOn = serverSettings?.debugRecording === true;
  const u = new URL(url);

  const handleToggle = async () => {
    if (!isAuthenticated || saving) return;
    setSaving(true);
    setError(null);
    try {
      await createServerSettingsRecord(!debugRecordingOn);
    } catch (e: any) {
      setError(e?.message ?? "Failed to update setting");
    } finally {
      setSaving(false);
    }
  };

  return (
    <div className="space-y-6">
      <h1 className="font-display text-xl font-semibold">
        {t("privacy-security")}
      </h1>

      {!isAuthenticated && (
        <p className="text-sm text-(--color-fg-muted)">
          {t("log-in-to-manage-privacy", {
            defaultValue: "Log in to manage privacy settings.",
          })}
        </p>
      )}

      <Card>
        <CardRow>
          <div className="flex items-center justify-between">
            <div className="pr-4">
              <div className="text-sm font-medium">
                {t("debug-recording-title", { host: u.host })}
              </div>
              <div className="mt-0.5 text-xs text-(--color-fg-muted)">
                {t("debug-recording-description")}
              </div>
            </div>
            <Switch
              checked={debugRecordingOn}
              onCheckedChange={handleToggle}
              disabled={!isAuthenticated || saving}
            />
          </div>
        </CardRow>
      </Card>

      {error && <p className="text-sm text-(--color-danger)">{error}</p>}
    </div>
  );
}
