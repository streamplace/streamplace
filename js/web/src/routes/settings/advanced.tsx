import { createFileRoute, useNavigate } from "@tanstack/react-router";
import { useEffect, useState } from "react";
import { useTranslation } from "react-i18next";
import { useStore } from "../../lib/store";
import { useStreamplaceUrl } from "../../lib/store/hooks";

export const Route = createFileRoute("/settings/advanced")({
  component: AdvancedSettings,
});

function AdvancedSettings() {
  const { t } = useTranslation("settings");
  const navigate = useNavigate();
  const url = useStreamplaceUrl();
  const setURL = useStore((s) => s.setURL);

  const defaultUrl =
    typeof window !== "undefined"
      ? window.location.origin.replace(/\/+$/, "")
      : "";

  const [overrideEnabled, setOverrideEnabled] = useState(false);
  const [newUrl, setNewUrl] = useState("");

  useEffect(() => {
    setOverrideEnabled(url !== defaultUrl);
  }, [url, defaultUrl]);

  const onSubmitUrl = () => {
    if (newUrl) {
      const trimmedUrl = newUrl.endsWith("/") ? newUrl.slice(0, -1) : newUrl;
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

      <h1 className="text-xl font-semibold">{t("advanced")}</h1>

      <div className="rounded-lg border border-[var(--color-border)] bg-[var(--color-bg-elevated)] p-4 space-y-4">
        {/* Custom node toggle */}
        <div className="flex items-center justify-between">
          <div>
            <div className="text-sm font-medium">{t("use-custom-node")}</div>
            <div className="text-xs text-[var(--color-fg-muted)] mt-0.5">
              {t("default-url", { url: defaultUrl })}
            </div>
          </div>
          <button
            type="button"
            role="switch"
            aria-checked={overrideEnabled}
            onClick={() => handleToggleOverride(!overrideEnabled)}
            className={`relative inline-flex h-5 w-9 shrink-0 cursor-pointer rounded-full border-2 border-transparent transition-colors ${
              overrideEnabled
                ? "bg-[var(--color-accent)]"
                : "bg-[var(--color-border)]"
            }`}
          >
            <span
              className={`pointer-events-none inline-block size-4 rounded-full bg-white shadow-sm transition-transform ${
                overrideEnabled ? "translate-x-4" : "translate-x-0"
              }`}
            />
          </button>
        </div>

        {/* URL input (shown when override enabled) */}
        {overrideEnabled && (
          <div className="flex gap-2">
            <input
              type="url"
              value={newUrl}
              onChange={(e) => setNewUrl(e.target.value)}
              placeholder={
                url !== defaultUrl ? url : t("enter-custom-node-url")
              }
              spellCheck={false}
              autoComplete="off"
              className="flex-1 h-9 rounded-lg border border-[var(--color-border)] bg-transparent px-3 text-sm font-mono outline-none focus:border-[var(--color-accent)]"
            />
            <button
              type="button"
              onClick={onSubmitUrl}
              disabled={!newUrl.trim()}
              className="h-9 px-4 rounded-md bg-[var(--color-accent)] hover:bg-[var(--color-accent-hover)] disabled:opacity-50 disabled:cursor-not-allowed text-[var(--color-accent-fg)] text-sm font-medium"
            >
              {t("save-button")}
            </button>
          </div>
        )}
      </div>
    </div>
  );
}
