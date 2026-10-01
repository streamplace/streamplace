import { Card, CardRow } from "@/components/ui/card";
import { Switch } from "@/components/ui/switch";
import { useBetaStatus } from "@/hooks/use-beta-status";
import { useSession } from "@/lib/session";
import { createFileRoute } from "@tanstack/react-router";
import { useEffect, useState } from "react";
import { useTranslation } from "react-i18next";
import { place } from "streamplace";
import { useStore } from "../../lib/store";
import {
  useIsReady,
  usePDSAgent,
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
  const agent = usePDSAgent();
  // Automatic VOD publishing is a preference the node keeps itself
  // (place.stream.server.getPreferences), not part of the settings record.
  const [autoPublishVods, setAutoPublishVods] = useState<boolean | null>(null);

  const isAuthenticated = sessionState.status === "authenticated";

  // The livestream-recording and automatic VOD publishing toggles are only
  // meaningful for accounts in the VOD beta — the node won't record anyone
  // else regardless of these flags — so we only surface them to them.
  const { status: vodBetaStatus } = useBetaStatus("vod");

  useEffect(() => {
    if (isReady && isAuthenticated) getServerSettingsFromPDS();
  }, [isReady, isAuthenticated]);

  useEffect(() => {
    if (!agent || !isAuthenticated) return;
    agent.client
      .call(place.stream.server.getPreferences)
      .then((res) => setAutoPublishVods(res.preferences.autoPublishVods))
      .catch((err) => console.error("Failed to load preferences:", err));
  }, [agent, isAuthenticated]);

  const debugRecordingOn = serverSettings?.debugRecording === true;
  // Defaults on (unlike debugRecording): only an explicit `false` turns it off.
  const livestreamRecordingOn = serverSettings?.livestreamRecording !== false;
  const u = new URL(url);

  const handleToggle = async (patch: {
    debugRecording?: boolean;
    livestreamRecording?: boolean;
  }) => {
    if (!isAuthenticated || saving) return;
    setSaving(true);
    setError(null);
    try {
      await createServerSettingsRecord(patch);
    } catch (error) {
      console.error("Failed to update privacy setting:", error);
      setError(
        t("privacy-update-failed", {
          defaultValue: "Failed to update setting",
        }),
      );
    } finally {
      setSaving(false);
    }
  };

  const handleAutoPublishVodsToggle = async (value: boolean) => {
    if (!agent || saving) return;
    setSaving(true);
    setError(null);
    try {
      const res = await agent.client.call(place.stream.server.putPreferences, {
        autoPublishVods: value,
      });
      setAutoPublishVods(res.preferences.autoPublishVods);
    } catch (error) {
      console.error("Failed to update preferences:", error);
      setError(t("auto-publish-vods-update-failed"));
    } finally {
      setSaving(false);
    }
  };

  return (
    <div className="space-y-6">
      <h1 className="font-display text-xl font-semibold">
        {t("privacy-security")}
      </h1>

      {!isAuthenticated ? (
        <p className="text-sm text-(--color-fg-muted)">
          {t("log-in-to-manage-privacy", {
            defaultValue: "Log in to manage privacy settings.",
          })}
        </p>
      ) : (
        <Card>
          <CardRow>
            <div className="flex items-center justify-between">
              <div className="pr-4">
                <div className="text-sm font-medium">
                  {t("debug-recording-title", { host: u.host })}
                </div>
              </div>
              <Switch
                checked={debugRecordingOn}
                onCheckedChange={(v) => handleToggle({ debugRecording: v })}
                disabled={!isAuthenticated || saving}
              />
            </div>
          </CardRow>
        </Card>
      )}

      {vodBetaStatus === "granted" && (
        <Card>
          <CardRow>
            <div className="flex items-center justify-between">
              <div className="pr-4">
                <div className="text-sm font-medium">
                  {t("livestream-recording-title", { host: u.host })}
                </div>
                <div className="mt-0.5 text-xs text-(--color-fg-muted)">
                  {t("livestream-recording-description")}
                </div>
              </div>
              <Switch
                checked={livestreamRecordingOn}
                onCheckedChange={(v) =>
                  handleToggle({ livestreamRecording: v })
                }
                disabled={!isAuthenticated || saving}
              />
            </div>
          </CardRow>
        </Card>
      )}

      {vodBetaStatus === "granted" && autoPublishVods !== null && (
        <Card>
          <CardRow>
            <div className="flex items-center justify-between">
              <div className="pr-4">
                <div className="text-sm font-medium">
                  {t("auto-publish-vods-title")}
                </div>
                <div className="mt-0.5 text-xs text-(--color-fg-muted)">
                  {t("auto-publish-vods-description", { host: u.host })}
                </div>
              </div>
              <Switch
                checked={autoPublishVods}
                onCheckedChange={handleAutoPublishVodsToggle}
                disabled={!isAuthenticated || saving}
              />
            </div>
          </CardRow>
        </Card>
      )}

      {error && (
        <p className="text-sm text-(--color-danger)" role="alert">
          {error}
        </p>
      )}
    </div>
  );
}
