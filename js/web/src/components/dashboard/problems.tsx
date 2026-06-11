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
      return <Info className="size-5 text-[var(--color-fg-muted)]" />;
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
      return "bg-[var(--color-bg-elevated)] border-[var(--color-border)]";
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
    <div className="h-full rounded-lg border border-[var(--color-border)] bg-[var(--color-bg-elevated)] overflow-hidden">
      <div className="flex items-center justify-between px-4 py-3 border-b border-[var(--color-border)]">
        <div className="flex items-center gap-2">
          <AlertTriangle className="size-4 text-amber-400" />
          <h3 className="text-sm font-semibold">
            {t("stream-optimization", { defaultValue: "Stream Optimization" })}
          </h3>
          {problems.length > 0 && (
            <span className="text-xs px-1.5 py-0.5 rounded-full bg-amber-500/20 text-amber-400 font-medium">
              {problems.length}
            </span>
          )}
        </div>
        <button
          type="button"
          onClick={() => setDismissed(true)}
          className="p-1 rounded hover:bg-[var(--color-bg)] text-[var(--color-fg-muted)] hover:text-[var(--color-fg)] transition-colors"
          aria-label={t("dismiss")}
        >
          <X className="size-4" />
        </button>
      </div>

      <div className="divide-y divide-[var(--color-border)]">
        {problems.map((problem: Problem) => (
          <div
            key={problem.code}
            className={`flex items-start gap-3 px-4 py-3 ${severityBg(problem.severity)}`}
          >
            <ProblemIcon severity={problem.severity} />
            <div className="flex-1 min-w-0">
              <div className="text-sm font-medium">{problem.code}</div>
              <div className="text-xs text-[var(--color-fg-muted)] mt-0.5">
                {problem.message}
              </div>
              {problem.link && (
                <a
                  href={problem.link}
                  target="_blank"
                  rel="noopener noreferrer"
                  className="text-xs text-[var(--color-accent)] hover:underline mt-1 inline-block"
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
