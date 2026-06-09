import { createFileRoute, Link, useNavigate } from "@tanstack/react-router";
import { Globe, Heart, Key, Webhook } from "lucide-react";
import { useTranslation } from "react-i18next";

export const Route = createFileRoute("/settings/streaming")({
  component: StreamingSettings,
});

interface StreamingLinkProps {
  to?: string;
  icon: React.ComponentType<{ size?: number; className?: string }>;
  label: string;
  description?: string;
  disabled?: boolean;
}

function StreamingLink({
  to,
  icon: Icon,
  label,
  description,
  disabled,
}: StreamingLinkProps) {
  const inner = (
    <div className="flex items-center justify-between w-full">
      <div className="flex items-center gap-3">
        <Icon size={20} className="text-[var(--color-fg-muted)]" />
        <div>
          <span className="text-sm">{label}</span>
          {description && (
            <div className="text-xs text-[var(--color-fg-muted)]">
              {description}
            </div>
          )}
        </div>
      </div>
      {disabled && (
        <span className="text-xs text-[var(--color-fg-muted)] px-2 py-0.5 rounded bg-[var(--color-bg)]">
          coming soon
        </span>
      )}
    </div>
  );

  if (to && !disabled) {
    return (
      <Link
        to={to}
        className="flex items-center px-3 py-2.5 hover:bg-[var(--color-bg)] transition-colors"
      >
        {inner}
      </Link>
    );
  }

  return (
    <div
      className={`flex items-center px-3 py-2.5 ${
        disabled ? "opacity-50" : ""
      }`}
    >
      {inner}
    </div>
  );
}

function StreamingSettings() {
  const { t } = useTranslation("settings");
  const navigate = useNavigate();

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

      <h1 className="text-xl font-semibold">{t("streaming")}</h1>

      <div className="rounded-lg border border-[var(--color-border)] bg-[var(--color-bg-elevated)] divide-y divide-[var(--color-border)]">
        <StreamingLink icon={Key} label={t("key-management")} disabled />
        <StreamingLink
          icon={Heart}
          label={t("recommendations-to-others")}
          disabled
        />
        <StreamingLink
          to="/settings/webhooks"
          icon={Webhook}
          label={t("webhooks")}
        />
        <StreamingLink icon={Globe} label={t("multistream")} disabled />
      </div>
    </div>
  );
}
