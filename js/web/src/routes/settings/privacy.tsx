import { createFileRoute, useNavigate } from "@tanstack/react-router";
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
  const navigate = useNavigate();
  const isReady = useIsReady();
  const serverSettings = useServerSettings();
  const url = useStreamplaceUrl();
  const getServerSettingsFromPDS = useStore((s) => s.getServerSettingsFromPDS);
  const createServerSettingsRecord = useStore(
    (s) => s.createServerSettingsRecord,
  );

  useEffect(() => {
    if (isReady) {
      getServerSettingsFromPDS();
    }
  }, [isReady]);

  const debugRecordingOn = serverSettings?.debugRecording === true;
  const u = new URL(url);

  return (
    <div className="space-y-6">
      <nav>
        <button
          type="button"
          onClick={() => navigate({ to: "/settings" })}
          className="flex items-center gap-2 text-sm text-[var(--color-fg-muted)] hover:text-[var(--color-fg)] transition-colors"
        >
          <svg width="16" height="16" viewBox="0 0 16 16" fill="none">
            <path
              d="M10 3l-5 5 5 5"
              stroke="currentColor"
              strokeWidth="1.5"
              strokeLinecap="round"
              strokeLinejoin="round"
            />
          </svg>
          {t("settings-title")}
        </button>
      </nav>

      <h1 className="text-xl font-semibold">{t("privacy-security")}</h1>

      <div className="rounded-lg border border-[var(--color-border)] bg-[var(--color-bg-elevated)] p-4">
        <div className="flex items-center justify-between">
          <div className="pr-4">
            <div className="text-sm font-medium">
              {t("debug-recording-title", { host: u.host })}
            </div>
            <div className="text-xs text-[var(--color-fg-muted)] mt-0.5">
              {t("debug-recording-description")}
            </div>
          </div>
          <button
            type="button"
            role="switch"
            aria-checked={debugRecordingOn}
            onClick={() => createServerSettingsRecord(!debugRecordingOn)}
            className={`relative inline-flex h-5 w-9 shrink-0 cursor-pointer rounded-full border-2 border-transparent transition-colors ${
              debugRecordingOn
                ? "bg-[var(--color-accent)]"
                : "bg-[var(--color-border)]"
            }`}
          >
            <span
              className={`pointer-events-none inline-block size-4 rounded-full bg-white shadow-sm transition-transform ${
                debugRecordingOn ? "translate-x-4" : "translate-x-0"
              }`}
            />
          </button>
        </div>
      </div>
    </div>
  );
}
