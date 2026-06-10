import { cn } from "@/lib/utils";
import {
  createFileRoute,
  Link,
  Outlet,
  useMatchRoute,
} from "@tanstack/react-router";
import {
  ChevronDown,
  Globe,
  Heart,
  Info,
  Key,
  Lock,
  Palette,
  Shield,
  User2,
  Video,
  Webhook,
} from "lucide-react";
import { useState } from "react";
import { useTranslation } from "react-i18next";

interface NavLink {
  to: string;
  icon: React.ComponentType<{ className?: string }>;
  labelKey: string;
}

interface NavGroup {
  icon: React.ComponentType<{ className?: string }>;
  labelKey: string;
  children: NavLink[];
}

type NavItem =
  | ({ role: "link" } & NavLink)
  | ({ role: "group" } & NavGroup)
  | { role: "divider" };

const NAV_ITEMS: NavItem[] = [
  { role: "link", to: "/settings/account", icon: User2, labelKey: "account" },
  {
    role: "link",
    to: "/settings/chat-profile",
    icon: Palette,
    labelKey: "chat-profile",
  },
  {
    role: "group",
    icon: Video,
    labelKey: "streaming",
    children: [
      { to: "/settings/keys", icon: Key, labelKey: "key-management" },
      {
        to: "/settings/recommendations",
        icon: Heart,
        labelKey: "recommendations-to-others",
      },
      { to: "/settings/webhooks", icon: Webhook, labelKey: "webhooks" },
      { to: "/settings/multistream", icon: Globe, labelKey: "multistream" },
    ],
  },
  {
    role: "link",
    to: "/settings/privacy",
    icon: Shield,
    labelKey: "privacy-security",
  },
  { role: "divider" },
  {
    role: "link",
    to: "/settings/languages",
    icon: Globe,
    labelKey: "languages",
  },
  { role: "link", to: "/settings/advanced", icon: Lock, labelKey: "advanced" },
  { role: "link", to: "/settings/about", icon: Info, labelKey: "about" },
];

const linkClass = cn(
  "flex items-center gap-2",
  "px-3 py-1.5 lg:rounded-md transition-colors -mr-0.5 lg:mr-0",
  "whitespace-nowrap",
  "text-(--color-fg-muted) hover:text-(--color-fg) hover:bg-(--color-bg-elevated)",
  "[&.active]:text-(--color-fg) [&.active]:font-medium",
  "lg:[&.active]:bg-(--color-bg-elevated)",
  "border-b [&.active]:border-b-(--color-fg)",
  "lg:border-b-0 lg:[&.active]:border-0",
);

const childLinkClass = cn(
  "flex items-center gap-2",
  "px-3 py-1 lg:rounded-md transition-colors text-sm",
  "text-(--color-fg-muted) hover:text-(--color-fg) hover:bg-(--color-bg-elevated)",
  "[&.active]:text-(--color-fg) [&.active]:font-medium",
  "lg:[&.active]:bg-(--color-bg-elevated)",
);

export const Route = createFileRoute("/settings")({
  component: SettingsLayout,
});

function NavGroupItem({ item }: { item: NavGroup }) {
  const { t } = useTranslation("settings");
  const matchRoute = useMatchRoute();
  const Icon = item.icon;

  // Check if any child route is active
  const isChildActive = item.children.some((child) =>
    matchRoute({ to: child.to, fuzzy: true }),
  );

  const [open, setOpen] = useState(isChildActive);

  return (
    <div>
      {/* Mobile: just show as a regular link to the streaming page */}
      <Link
        to="/settings/streaming"
        className={cn(
          linkClass,
          "lg:hidden",
          isChildActive && "text-(--color-fg) font-medium",
        )}
      >
        {t(item.labelKey)}
      </Link>

      {/* Desktop: expandable group */}
      <div className="hidden lg:block">
        <button
          type="button"
          onClick={() => setOpen(!open)}
          className={cn(
            linkClass,
            "w-full justify-between",
            isChildActive && "text-(--color-fg) font-medium",
          )}
        >
          <span className="flex items-center gap-2">
            <Icon className="size-4" />
            {t(item.labelKey)}
          </span>
          <ChevronDown
            className={cn(
              "size-3.5 transition-transform",
              open && "rotate-180",
            )}
          />
        </button>
        {open && (
          <div className="ml-4.5 border-l border-(--color-border) space-y-0.5 mt-0.5 mb-1">
            {item.children.map((child) => (
              <Link key={child.to} to={child.to} className={childLinkClass}>
                <child.icon className="size-3.5" />
                {t(child.labelKey)}
              </Link>
            ))}
          </div>
        )}
      </div>
    </div>
  );
}

function DisplayNavItem({ item }: { item: NavItem }) {
  if (item.role === "divider") {
    return (
      <div className="border-b border-(--color-border) my-0.5 lg:mx-2 lg:border-b-0 lg:my-2" />
    );
  }

  if (item.role === "group") {
    return <NavGroupItem item={item} />;
  }

  const { t } = useTranslation("settings");
  const Icon = item.icon;

  return (
    <Link to={item.to} className={linkClass}>
      <Icon className="size-4 hidden lg:block" />
      {t(item.labelKey)}
    </Link>
  );
}

function SettingsLayout() {
  const { t } = useTranslation("settings");

  return (
    <div className="flex flex-col lg:flex-row gap-6 lg:gap-8 px-4 lg:px-6 py-6 lg:py-10 mx-auto max-w-screen">
      {/* Mobile: floating sticky nav */}
      <div className="lg:hidden sticky top-0 z-30 -mx-4 px-4 py-2 bg-[var(--color-bg)]/80 backdrop-blur-md">
        <h2 className="text-lg font-semibold tracking-tight mb-2 font-display">
          {t("settings-title")}
        </h2>

        <div className="relative -mx-4 px-4">
          <div className="pointer-events-none absolute left-0 top-0 bottom-0 w-5 bg-gradient-to-r from-[var(--color-bg)] to-transparent z-10" />
          <div className="pointer-events-none absolute right-0 top-0 bottom-0 w-5 bg-gradient-to-l from-[var(--color-bg)] to-transparent z-10" />

          <nav
            className="flex gap-0.5 overflow-x-auto scrollbar-none -mx-5 px-5"
            style={{ scrollbarWidth: "none", msOverflowStyle: "none" }}
          >
            {NAV_ITEMS.map((item, i) => (
              <DisplayNavItem
                key={"labelKey" in item ? item.labelKey : `divider-${i}`}
                item={item}
              />
            ))}
          </nav>
        </div>
      </div>

      {/* Desktop: sidebar */}
      <div className="hidden lg:block lg:w-52 shrink-0">
        <nav className="flex flex-col gap-0.5 sticky top-6 self-start">
          <p className="text-2xl font-semibold px-2 pb-3 font-display">
            {t("settings-title")}
          </p>
          {NAV_ITEMS.map((item, i) => (
            <DisplayNavItem
              key={"labelKey" in item ? item.labelKey : `divider-${i}`}
              item={item}
            />
          ))}
        </nav>
      </div>

      <div className="flex-1 min-w-0 md:max-w-md xl:max-w-xl w-full md:w-screen mt-0.75">
        <Outlet />
      </div>
    </div>
  );
}
