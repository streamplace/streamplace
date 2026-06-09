import { Card, CardRow } from "@/components/ui/card";
import { Switch } from "@/components/ui/switch";
import { createFileRoute } from "@tanstack/react-router";
import { useEffect, useState } from "react";
import { useTranslation } from "react-i18next";
import { useStore } from "../../lib/store";
import { useStreamplaceUrl } from "../../lib/store/hooks";

export const Route = createFileRoute("/settings/advanced")({
  component: AdvancedSettings,
});

function AdvancedSettings() {
  const { t } = useTranslation("settings");
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
      setURL(newUrl.endsWith("/") ? newUrl.slice(0, -1) : newUrl);
      setNewUrl("");
    }
  };

  const handleToggle = (enabled: boolean) => {
    setOverrideEnabled(enabled);
    if (!enabled) setURL(defaultUrl);
  };

  return (
    <div className="space-y-6">
      <h1 className="text-xl font-semibold font-display">{t("advanced")}</h1>

      <Card>
        <CardRow>
          <div className="flex items-center justify-between">
            <div>
              <div className="text-sm font-medium">{t("use-custom-node")}</div>
              <div className="text-xs text-[var(--color-fg-muted)] mt-0.5">
                {t("default-url", { url: defaultUrl })}
              </div>
            </div>
            <Switch checked={overrideEnabled} onCheckedChange={handleToggle} />
          </div>
        </CardRow>

        {overrideEnabled && (
          <CardRow>
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
          </CardRow>
        )}
      </Card>
    </div>
  );
}
