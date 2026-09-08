import { Button } from "@/components/ui/button";
import { CardMenuSection } from "@/components/ui/card";
import { createFileRoute } from "@tanstack/react-router";
import { ExternalLink } from "lucide-react";
import { useState } from "react";
import { useTranslation } from "react-i18next";
import { useDevMode } from "../../hooks/use-dev-mode";
import { useStore } from "../../lib/store";

export const Route = createFileRoute("/settings/about")({
  component: AboutSettings,
});

const UNLOCK_TAP_COUNT = 5;

function AboutSettings() {
  const { t } = useTranslation("settings");
  const [tapCount, setTapCount] = useState(0);
  const isDev = typeof import.meta !== "undefined" && import.meta.env?.DEV;
  const [devMode, toggleDevMode] = useDevMode();
  const danmuUnlocked = useStore((s) => s.danmuUnlocked);
  const setDanmuUnlocked = useStore((s) => s.setDanmuUnlocked);

  const handleVersionPress = () => {
    if (danmuUnlocked) {
      // Already unlocked; tapping again doesn't toggle it off. The app
      // allows disabling from the toast action; keep it simple here.
      return;
    }
    const newCount = tapCount + 1;
    setTapCount(newCount);
    if (newCount >= UNLOCK_TAP_COUNT) {
      toggleDevMode();
      setDanmuUnlocked(true);
      setTapCount(0);
    }
  };

  const isStreamplace =
    typeof window !== "undefined" &&
    (window.location.hostname.endsWith("stream.place") ||
      window.location.hostname.endsWith(".stream.place"));

  return (
    <div className="space-y-6">
      <h1 className="font-display text-xl font-semibold">{t("about")}</h1>

      <CardMenuSection>
        <div className="flex items-center justify-between px-3 py-2.5">
          <span className="text-sm">Streamplace</span>
          <span className="text-sm text-(--color-fg-muted)">v0.0.0</span>
        </div>
        <Button
          type="button"
          onClick={handleVersionPress}
          variant="ghost"
          className="h-auto w-full justify-between rounded-none px-3 py-2.5 text-left"
        >
          <span className="text-sm">
            {t("build", { defaultValue: "Build" })}
          </span>
          <span className="text-sm text-(--color-fg-muted)">
            {isDev
              ? t("build-development", { defaultValue: "dev" })
              : t("build-production", { defaultValue: "prod" })}
          </span>
        </Button>
      </CardMenuSection>

      {(devMode || isDev) && (
        <CardMenuSection>
          <div className="flex items-center justify-between px-3 py-2.5">
            <span className="text-sm">
              {t("developer-mode", { defaultValue: "Developer Mode" })}
            </span>
            <span className="rounded bg-green-500/20 px-2 py-0.5 font-mono text-xs text-green-400">
              active
            </span>
          </div>
        </CardMenuSection>
      )}

      {isStreamplace && (
        <CardMenuSection>
          <a
            href="https://privacy.stream.place"
            target="_blank"
            rel="noopener noreferrer"
            className="flex items-center justify-between px-3 py-2.5 transition-colors hover:bg-(--color-bg)"
          >
            <span className="text-sm">
              {t("privacy-policy", { defaultValue: "Privacy Policy" })}
            </span>
            <ExternalLink className="size-4 text-(--color-fg-muted)" />
          </a>
        </CardMenuSection>
      )}
    </div>
  );
}
