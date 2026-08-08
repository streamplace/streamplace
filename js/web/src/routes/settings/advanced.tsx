import { Button } from "@/components/ui/button";
import { Card, CardRow } from "@/components/ui/card";
import { Switch } from "@/components/ui/switch";
import { createFileRoute } from "@tanstack/react-router";
import { Check, Loader2 } from "lucide-react";
import { useEffect, useState } from "react";
import { useTranslation } from "react-i18next";
import { useStore } from "../../lib/store";
import { useStreamplaceUrl } from "../../lib/store/hooks";
import { isWebBetaEnabled, setWebBetaEnabled } from "../../lib/web-beta";

export const Route = createFileRoute("/settings/advanced")({
  component: AdvancedSettings,
});

type RefreshStatus = "ready" | "active" | "done";

function AdvancedSettings() {
  const { t } = useTranslation("settings");
  const url = useStreamplaceUrl();
  const setURL = useStore((s) => s.setURL);
  const fetchBranding = useStore((s) => s.fetchBranding);
  const defaultUrl =
    typeof window !== "undefined"
      ? window.location.origin.replace(/\/+$/, "")
      : "";

  const [overrideEnabled, setOverrideEnabled] = useState(false);
  const [newUrl, setNewUrl] = useState("");
  const [webBeta, setWebBeta] = useState(false);
  const [refreshBranding, setRefreshBranding] =
    useState<RefreshStatus>("ready");

  useEffect(() => {
    setOverrideEnabled(url !== defaultUrl);
  }, [url, defaultUrl]);

  useEffect(() => {
    setWebBeta(isWebBetaEnabled());
  }, []);

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

  // Toggling web-beta writes a cookie and reloads. The reload is what
  // actually flips the user over to the other frontend; the server
  // reads the cookie on the next request and picks the matching bundle.
  const handleWebBetaToggle = (enabled: boolean) => {
    setWebBetaEnabled(enabled);
    if (typeof window !== "undefined") {
      window.location.reload();
    }
  };

  const handleRefreshBranding = () => {
    setRefreshBranding("active");
    fetchBranding({ force: true }).then(() => {
      setRefreshBranding("done");
      // set back to ready after a short delay
      setTimeout(() => {
        setRefreshBranding("ready");
      }, 2500);
    });
  };

  return (
    <div className="space-y-6">
      <h1 className="font-display text-xl font-semibold">{t("advanced")}</h1>

      <Card>
        <CardRow>
          <div className="flex items-center justify-between">
            <div>
              <div className="text-sm font-medium">{t("use-custom-node")}</div>
              <div className="mt-0.5 text-xs text-(--color-fg-muted)">
                {t("default-url", { url: defaultUrl })}
              </div>
            </div>
            <Switch checked={overrideEnabled} onCheckedChange={handleToggle} />
          </div>
        </CardRow>

        {overrideEnabled && (
          <CardRow>
            <div className="flex items-center justify-center gap-2">
              <input
                type="url"
                value={newUrl}
                onChange={(e) => setNewUrl(e.target.value)}
                placeholder={
                  url !== defaultUrl ? url : t("enter-custom-node-url")
                }
                spellCheck={false}
                autoComplete="off"
                className="h-9 flex-1 rounded-lg border border-(--color-border) bg-transparent px-3 font-mono text-sm outline-none focus:border-(--color-accent)"
              />
              <Button
                type="button"
                size="lg"
                onClick={onSubmitUrl}
                disabled={!newUrl.trim()}
              >
                {t("save-button")}
              </Button>
            </div>
          </CardRow>
        )}
      </Card>

      <Card>
        <CardRow>
          <div className="flex items-center justify-between gap-4">
            <div>
              <div className="text-sm font-medium">{t("try-new-web")}</div>
              <div className="mt-0.5 text-xs text-(--color-fg-muted)">
                {t("try-new-web-description")}
              </div>
            </div>
            <Switch checked={webBeta} onCheckedChange={handleWebBetaToggle} />
          </div>
        </CardRow>
      </Card>

      <Card>
        <button
          type="button"
          onClick={handleRefreshBranding}
          className="flex w-full items-center justify-between px-3 py-2.5 text-left transition-colors hover:bg-(--color-bg)"
        >
          <span className="text-sm">{t("refresh-branding")}</span>
          {refreshBranding === "active" && (
            <Loader2 className="size-4 animate-spin text-(--color-fg-muted)" />
          )}
          {refreshBranding === "done" && (
            <Check className="size-4 text-green-400" />
          )}
        </button>
      </Card>
    </div>
  );
}
