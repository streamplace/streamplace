import { BackLink } from "@/components/settings/back-link";
import { CardDivide } from "@/components/ui/card";
import { createFileRoute, Link } from "@tanstack/react-router";
import { Globe, Heart, Key, Webhook } from "lucide-react";
import { useTranslation } from "react-i18next";

export const Route = createFileRoute("/settings/streaming")({
  component: StreamingSettings,
});

interface StreamingLinkProps {
  to?: string;
  icon: React.ComponentType<{ size?: number; className?: string }>;
  label: string;
  disabled?: boolean;
}

function StreamingLink({
  to,
  icon: Icon,
  label,
  disabled,
}: StreamingLinkProps) {
  const inner = (
    <div className="flex items-center justify-between w-full">
      <div className="flex items-center gap-3">
        <Icon size={20} className="text-[var(--color-fg-muted)]" />
        <span className="text-sm">{label}</span>
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
      className={`flex items-center px-3 py-2.5 ${disabled ? "opacity-50" : ""}`}
    >
      {inner}
    </div>
  );
}

function StreamingSettings() {
  const { t } = useTranslation("settings");

  return (
    <div className="space-y-6">
      <BackLink to="/settings" label={t("settings-title")} />
      <h1 className="text-xl font-semibold">{t("streaming")}</h1>

      <CardDivide>
        <StreamingLink
          to="/settings/keys"
          icon={Key}
          label={t("key-management")}
        />
        <StreamingLink
          to="/settings/recommendations"
          icon={Heart}
          label={t("recommendations-to-others")}
        />
        <StreamingLink
          to="/settings/webhooks"
          icon={Webhook}
          label={t("webhooks")}
        />
        <StreamingLink
          to="/settings/multistream"
          icon={Globe}
          label={t("multistream")}
        />
      </CardDivide>
    </div>
  );
}
