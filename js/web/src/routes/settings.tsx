import { useSession } from "@/lib/session";
import { useStore } from "@/lib/store";
import { useUserProfile } from "@/lib/store/hooks";
import { cn } from "@/lib/utils";
import {
  createFileRoute,
  Link,
  Outlet,
  useMatchRoute,
} from "@tanstack/react-router";
import {
  Bell,
  ChevronDown,
  Globe,
  Info,
  Lock,
  Palette,
  Shield,
  User2,
  Video,
} from "lucide-react";
import { useState } from "react";
import { useTranslation } from "react-i18next";

interface NavLink {
  needsAuth?: boolean;
  /** Only show when danmu is unlocked (about-page easter egg). */
  requiresDanmuUnlock?: boolean;
  to: string;
  icon: React.ComponentType<{ className?: string }>;
  labelKey: string;
}

interface NavGroup {
  needsAuth?: boolean;
  icon: React.ComponentType<{ className?: string }>;
  labelKey: string;
  children: NavLink[];
}

type NavItem =
  | ({ role: "link" } & NavLink)
  | ({ role: "group" } & NavGroup)
  | { role: "divider" };

const NAV_ITEMS: NavItem[] = [
  {
    role: "link",
    to: "/settings/account",
    icon: User2,
    labelKey: "account",
    needsAuth: true,
  },
  {
    role: "link",
    needsAuth: true,
    to: "/settings/chat-profile",
    icon: Palette,
    labelKey: "chat-profile",
  },
  {
    role: "link",
    to: "/settings/privacy",
    icon: Shield,
    labelKey: "privacy-security",
  },
  {
    role: "link",
    needsAuth: true,
    to: "/settings/notifications",
    icon: Bell,
    labelKey: "notifications",
  },
  { role: "divider" },
  {
    role: "link",
    needsAuth: true,
    to: "/settings/danmu",
    icon: Palette,
    labelKey: "danmu",
    requiresDanmuUnlock: true,
  },
  {
    role: "link",
    to: "/settings/languages",
    icon: Globe,
    labelKey: "languages",
  },
  { role: "link", to: "/settings/advanced", icon: Lock, labelKey: "advanced" },
  { role: "link", to: "/settings/about", icon: Info, labelKey: "about" },
  {
    needsAuth: true,
    role: "link",
    to: "/dashboard/stream",
    icon: Video,
    labelKey: "creator-settings",
  },
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
      {/* Desktop: expandable group */}
      <div className="hidden lg:block">
        <button
          type="button"
          onClick={() => setOpen(!open)}
          className={cn(
            linkClass,
            "w-full justify-between",
            isChildActive && "font-medium text-(--color-fg)",
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
          <div className="mt-0.5 mb-1 ml-4.5 space-y-0.5 border-l border-(--color-border)">
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
  const { t } = useTranslation("settings");
  // get auth state from context or store
  const { state } = useSession();
  const userProfile = useUserProfile();
  const danmuUnlocked = useStore((s) => s.danmuUnlocked);
  if (state.status !== "authenticated" || !userProfile) {
    // if the item requires auth and we're not on that page, don't render it
    if ("needsAuth" in item && item.needsAuth) {
      return null;
    }
  }
  if ("requiresDanmuUnlock" in item && item.requiresDanmuUnlock) {
    if (!danmuUnlocked) return null;
  }
  if (item.role === "divider") {
    return (
      <div className="my-0.5 border-b border-(--color-border) lg:mx-2 lg:my-2 lg:border-b-0" />
    );
  }

  if (item.role === "group") {
    return <NavGroupItem item={item} />;
  }

  const Icon = item.icon;

  return (
    <Link to={item.to} className={linkClass}>
      <Icon className="hidden size-4 lg:block" />
      {t(item.labelKey)}
    </Link>
  );
}

function SettingsLayout() {
  const { t } = useTranslation("settings");

  return (
    <div className="mx-auto flex max-w-screen flex-col gap-6 px-4 py-6 lg:flex-row lg:gap-8 lg:px-6 lg:py-10">
      {/* Mobile: floating sticky nav */}
      <div className="sticky top-0 z-30 -mx-4 bg-(--color-bg)/80 px-4 py-2 backdrop-blur-md lg:hidden">
        <h2 className="font-display mb-2 text-lg font-semibold tracking-tight">
          {t("settings-title")}
        </h2>

        <div className="relative -mx-4 px-4">
          <div className="pointer-events-none absolute top-0 bottom-0 left-0 z-10 w-5 bg-linear-to-r from-(--color-bg) to-transparent" />
          <div className="pointer-events-none absolute top-0 right-0 bottom-0 z-10 w-5 bg-linear-to-l from-(--color-bg) to-transparent" />

          <nav
            className="-mx-5 flex scrollbar-none gap-0.5 overflow-x-auto px-5"
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
      <div className="hidden shrink-0 lg:block lg:w-56">
        <nav className="sticky top-6 flex flex-col gap-0.5 self-start">
          <p className="font-display px-2 pb-3 text-2xl font-semibold">
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

      <div className="mt-0.75 w-full min-w-0 flex-1 md:w-screen md:max-w-md xl:max-w-xl">
        <Outlet />
      </div>
    </div>
  );
}
