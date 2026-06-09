import { cn } from "@/lib/utils";
import { createFileRoute, Link, Outlet } from "@tanstack/react-router";
import { Globe, Info, Lock, Shield, User2, Video } from "lucide-react";
import { useTranslation } from "react-i18next";

const NAV_ITEMS = [
  { to: "/settings/account", icon: User2, labelKey: "account" },
  { to: "/settings/streaming", icon: Video, labelKey: "streaming" },
  { to: "/settings/privacy", icon: Shield, labelKey: "privacy-security" },
  { to: "/settings/languages", icon: Globe, labelKey: "languages" },
  { to: "/settings/advanced", icon: Lock, labelKey: "advanced" },
  { to: "/settings/about", icon: Info, labelKey: "about" },
] as const;

export const Route = createFileRoute("/settings")({
  component: SettingsLayout,
});

function SettingsLayout() {
  const { t } = useTranslation("settings");

  return (
    <div className="flex flex-col md:flex-row gap-6 md:gap-8 px-6 py-10">
      <h2 className="text-2xl md:hidden font-semibold tracking-tight ml-2">
        {t("settings-title")}
      </h2>

      <nav className="flex flex-row md:flex-col gap-0.5 shrink-0 md:w-40 md:sticky md:top-6 md:self-start">
        <p className="hidden md:block text-2xl font-semibold px-2 pb-3">
          {t("settings-title")}
        </p>
        {NAV_ITEMS.map(({ to, icon: Icon, labelKey }) => (
          <Link
            key={to}
            to={to}
            className={cn(
              "flex items-center gap-2",
              "text-sm px-3 py-1.5 md:rounded-md transition-colors -mr-0.5 md:mr-0",
              "text-[var(--color-fg-muted)] hover:text-[var(--color-fg)] hover:bg-[var(--color-bg-elevated)]",
              "[&.active]:text-[var(--color-fg)] [&.active]:font-medium",
              "md:[&.active]:bg-[var(--color-bg-elevated)]",
              "border-b [&.active]:border-b-[var(--color-fg)]",
              "md:border-b-0 md:[&.active]:border-0",
            )}
          >
            <Icon className="size-4 hidden md:block" />
            {t(labelKey)}
          </Link>
        ))}
        <div className="md:hidden flex-1 border-b border-[var(--color-border)] self-end" />
      </nav>

      <div className="flex-1 min-w-0">
        <Outlet />
      </div>
    </div>
  );
}
