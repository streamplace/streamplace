import type { LivestreamStore } from "@streamplace/core";
import { AlertCircle, AlertTriangle, Info, X } from "lucide-react";
import { useState } from "react";
import { useTranslation } from "react-i18next";
import { useStore } from "zustand";

interface Problem {
  code: string;
  severity: string;
  message: string;
  link?: string;
}

function ProblemIcon({ severity }: { severity: string }) {
  switch (severity) {
    case "error":
      return <AlertCircle className="size-5 text-red-400" />;
    case "warning":
      return <AlertTriangle className="size-5 text-amber-400" />;
    case "info":
      return <Info className="size-5 text-blue-400" />;
    default:
      return <Info className="size-5 text-(--color-fg-muted)" />;
  }
}

function severityBg(severity: string): string {
  switch (severity) {
    case "error":
      return "bg-red-500/10 border-red-500/20";
    case "warning":
      return "bg-amber-500/10 border-amber-500/20";
    case "info":
      return "bg-blue-500/10 border-blue-500/20";
    default:
      return "bg-(--color-bg-elevated) border-(--color-border)";
  }
}

/**
 * Displays stream optimization warnings from the LivestreamStore.
 * Renders as a card widget for the dashboard control panel.
 */
export function ProblemsWidget({ store }: { store: LivestreamStore }) {
  const { t } = useTranslation("common");
  const problems = useStore(store, (s) => s.problems);
  const [dismissed, setDismissed] = useState(false);

  if (problems.length === 0 || dismissed) {
    return null;
  }

  return (
    <div className="h-full overflow-hidden rounded-lg border border-(--color-border) bg-(--color-bg-elevated)">
      <div className="flex items-center justify-between border-b border-(--color-border) px-4 py-3">
        <div className="flex items-center gap-2">
          <AlertTriangle className="size-4 text-amber-400" />
          <h3 className="text-sm font-semibold">
            {t("stream-optimization", { defaultValue: "Stream Optimization" })}
          </h3>
          {problems.length > 0 && (
            <span className="rounded-full bg-amber-500/20 px-1.5 py-0.5 text-xs font-medium text-amber-400">
              {problems.length}
            </span>
          )}
        </div>
        <button
          type="button"
          onClick={() => setDismissed(true)}
          className="rounded p-1 text-(--color-fg-muted) transition-colors hover:bg-(--color-bg) hover:text-(--color-fg)"
          aria-label={t("dismiss")}
        >
          <X className="size-4" />
        </button>
      </div>

      <div className="divide-y divide-(--color-border)">
        {problems.map((problem: Problem) => (
          <div
            key={problem.code}
            className={`flex items-start gap-3 px-4 py-3 ${severityBg(problem.severity)}`}
          >
            <ProblemIcon severity={problem.severity} />
            <div className="min-w-0 flex-1">
              <div className="text-sm font-medium">{problem.code}</div>
              <div className="mt-0.5 text-xs text-(--color-fg-muted)">
                {problem.message}
              </div>
              {problem.link && (
                <a
                  href={problem.link}
                  target="_blank"
                  rel="noopener noreferrer"
                  className="mt-1 inline-block text-xs text-(--color-accent) hover:underline"
                >
                  Learn more
                </a>
              )}
            </div>
          </div>
        ))}
      </div>
    </div>
  );
}
