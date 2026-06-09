import { cn } from "@/lib/utils";
import { createFileRoute, Link, Outlet } from "@tanstack/react-router";
import { Globe, Info, Lock, Shield, User2, Video } from "lucide-react";
import { useTranslation } from "react-i18next";

type NavItem =
  | {
      role: "link";
      to: string;
      icon: React.ComponentType<{ className?: string }>;
      labelKey: string;
    }
  | { role: "divider"; labelKey: string };

const NAV_ITEMS = [
  { role: "link", to: "/settings/account", icon: User2, labelKey: "account" },
  {
    role: "link",
    to: "/settings/streaming",
    icon: Video,
    labelKey: "streaming",
  },
  {
    role: "link",
    to: "/settings/privacy",
    icon: Shield,
    labelKey: "privacy-security",
  },
  { role: "divider", labelKey: "divider1" },
  {
    role: "link",
    to: "/settings/languages",
    icon: Globe,
    labelKey: "languages",
  },
  { role: "link", to: "/settings/advanced", icon: Lock, labelKey: "advanced" },
  { role: "link", to: "/settings/about", icon: Info, labelKey: "about" },
] as const;

export const Route = createFileRoute("/settings")({
  component: SettingsLayout,
});

function DisplayNavItem({ item }: { item: NavItem }) {
  const { t } = useTranslation("settings");
  if (item.role === "divider") {
    return <div className="border-b border-(--color-border) my-1" />;
  }
  // item must be a link if not divider

  const Icon = item.icon;

  return (
    <Link
      to={item.to}
      className={cn(
        "flex items-center gap-2",
        "text-sm px-3 py-1.5 md:rounded-md transition-colors -mr-0.5 md:mr-0",
        "text-(--color-fg-muted) hover:text-(--color-fg) hover:bg-(--color-bg-elevated)",
        "[&.active]:text-(--color-fg) [&.active]:font-medium",
        "md:[&.active]:bg-(--color-bg-elevated)",
        "border-b [&.active]:border-b-(--color-fg)",
        "md:border-b-0 md:[&.active]:border-0",
      )}
    >
      <Icon className="size-4 hidden md:block" />
      {t(item.labelKey)}
    </Link>
  );
}

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
        {NAV_ITEMS.map((item) => (
          <DisplayNavItem key={item.labelKey} item={item} />
        ))}
        <div className="md:hidden flex-1 border-b border-(--color-border) self-end" />
      </nav>

      <div className="flex-1 min-w-0 max-w-lg mx-auto">
        <Outlet />
      </div>
    </div>
  );
}
