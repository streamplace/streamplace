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
  "px-3 py-1.5 md:rounded-md transition-colors -mr-0.5 md:mr-0",
  "text-(--color-fg-muted) hover:text-(--color-fg) hover:bg-(--color-bg-elevated)",
  "[&.active]:text-(--color-fg) [&.active]:font-medium",
  "md:[&.active]:bg-(--color-bg-elevated)",
  "border-b [&.active]:border-b-(--color-fg)",
  "md:border-b-0 md:[&.active]:border-0",
);

const childLinkClass = cn(
  "flex items-center gap-2",
  "px-3 py-1 md:rounded-md transition-colors text-sm",
  "text-(--color-fg-muted) hover:text-(--color-fg) hover:bg-(--color-bg-elevated)",
  "[&.active]:text-(--color-fg) [&.active]:font-medium",
  "md:[&.active]:bg-(--color-bg-elevated)",
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
          <Icon className="size-4 hidden md:block" />
          {t(item.labelKey)}
        </span>
        <ChevronDown
          className={cn("size-3.5 transition-transform", open && "rotate-180")}
        />
      </button>
      {open && (
        <div className="md:pl-2 md:ml-3 md:border-l md:border-(--color-border) space-y-0.5 mt-0.5 mb-1">
          {item.children.map((child) => (
            <Link key={child.to} to={child.to} className={childLinkClass}>
              <child.icon className="size-3.5 hidden md:block" />
              {t(child.labelKey)}
            </Link>
          ))}
        </div>
      )}
    </div>
  );
}

function DisplayNavItem({ item }: { item: NavItem }) {
  if (item.role === "divider") {
    return (
      <div className="border-b border-(--color-border) my-0.5 mx-2 md:mx-0 md:border-b-0 md:my-2" />
    );
  }

  if (item.role === "group") {
    return <NavGroupItem item={item} />;
  }

  const { t } = useTranslation("settings");
  const Icon = item.icon;

  return (
    <Link to={item.to} className={linkClass}>
      <Icon className="size-4 hidden md:block" />
      {t(item.labelKey)}
    </Link>
  );
}

function SettingsLayout() {
  const { t } = useTranslation("settings");

  return (
    <div className="flex flex-col md:flex-row gap-6 md:gap-8 px-6 py-10 mx-auto">
      <h2 className="text-2xl md:hidden font-semibold tracking-tight ml-2">
        {t("settings-title")}
      </h2>

      <div className="relative md:w-52 shrink-0">
        {/* gradient fades for mobile scroll hint */}
        <div className="pointer-events-none absolute left-0 top-0 bottom-0 w-6 bg-gradient-to-r from-[var(--color-bg)] to-transparent z-10 md:hidden" />
        <div className="pointer-events-none absolute right-0 top-0 bottom-0 w-6 bg-gradient-to-l from-[var(--color-bg)] to-transparent z-10 md:hidden" />

        <nav
          className="flex md:flex-col gap-0.5 overflow-x-auto scrollbar-none md:overflow-visible md:sticky md:top-0 md:self-start"
          style={{ scrollbarWidth: "none", msOverflowStyle: "none" }}
        >
          <p className="hidden md:block text-2xl font-semibold px-2 pb-3">
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

      <div className="flex-1 min-w-0 max-w-2xl w-screen mx-auto md:mx-0">
        <Outlet />
      </div>
    </div>
  );
}
