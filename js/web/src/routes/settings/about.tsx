import { CardMenuSection } from "@/components/ui/card";
import { createFileRoute } from "@tanstack/react-router";
import { useState } from "react";
import { useTranslation } from "react-i18next";
import { useDevMode } from "../../hooks/use-dev-mode";

export const Route = createFileRoute("/settings/about")({
  component: AboutSettings,
});

const UNLOCK_TAP_COUNT = 5;

function AboutSettings() {
  const { t } = useTranslation("settings");
  const [tapCount, setTapCount] = useState(0);
  const isDev = typeof import.meta !== "undefined" && import.meta.env?.DEV;
  const [devMode, toggleDevMode] = useDevMode();

  const handleVersionPress = () => {
    const newCount = tapCount + 1;
    setTapCount(newCount);
    if (newCount >= UNLOCK_TAP_COUNT) {
      toggleDevMode();
      setTapCount(0);
    }
  };

  const isStreamplace =
    typeof window !== "undefined" &&
    (window.location.hostname.endsWith("stream.place") ||
      window.location.hostname.endsWith(".stream.place"));

  return (
    <div className="space-y-6">
      <h1 className="text-xl font-semibold font-display">{t("about")}</h1>

      <CardMenuSection>
        <div className="flex items-center justify-between px-3 py-2.5">
          <span className="text-sm">Streamplace</span>
          <span className="text-sm text-[var(--color-fg-muted)]">v0.0.0</span>
        </div>
        <button
          type="button"
          onClick={handleVersionPress}
          className="flex items-center justify-between px-3 py-2.5 w-full hover:bg-[var(--color-bg)] transition-colors"
        >
          <span className="text-sm">Build</span>
          <span className="text-sm text-[var(--color-fg-muted)]">
            {isDev ? "dev" : "prod"}
          </span>
        </button>
      </CardMenuSection>

      {(devMode || isDev) && (
        <CardMenuSection>
          <div className="flex items-center justify-between px-3 py-2.5">
            <span className="text-sm">Developer Mode</span>
            <span className="text-xs px-2 py-0.5 rounded bg-green-500/20 text-green-400 font-mono">
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
            className="flex items-center justify-between px-3 py-2.5 hover:bg-[var(--color-bg)] transition-colors"
          >
            <span className="text-sm">Privacy Policy</span>
            <svg
              width="16"
              height="16"
              viewBox="0 0 16 16"
              fill="none"
              className="text-[var(--color-fg-muted)]"
            >
              <path
                d="M12 9V13a1 1 0 01-1 1H3a1 1 0 01-1-1V5a1 1 0 011-1h4M10 2h4v4M14 2L7 9"
                stroke="currentColor"
                strokeWidth="1.5"
                strokeLinecap="round"
                strokeLinejoin="round"
              />
            </svg>
          </a>
        </CardMenuSection>
      )}
    </div>
  );
}
