import { LivestreamProvider } from "@/components/stream/livestream-provider";
import { useFullscreen } from "@/contexts/fullscreen-context";
import { useSession } from "@/lib/session";
import { Link, Outlet } from "@tanstack/react-router";
import {
  ArrowLeft,
  ArrowUpFromLine,
  Key,
  LayoutGrid,
  ListVideo,
  Radio,
  Share2,
  Video,
  Webhook,
} from "lucide-react";
import { useState } from "react";
import { useTranslation } from "react-i18next";
import {
  Sidebar,
  SidebarContent,
  SidebarGroup,
  SidebarGroupContent,
  SidebarGroupLabel,
  SidebarInset,
  SidebarMenu,
  SidebarMenuButton,
  SidebarMenuItem,
  SidebarProvider,
  SidebarTrigger,
} from "../ui/sidebar";
import { DashboardMetricsProvider } from "./dashboard-metrics";
import { DashboardStoreContext } from "./dashboard-store-context";

interface NavLink {
  to: string;
  icon: React.ComponentType<{ className?: string }>;
  labelKey: string;
}

/** Main nav — appears above the Settings group. */
const MAIN_NAV: NavLink[] = [
  { to: "/dashboard", icon: LayoutGrid, labelKey: "control-panel" },
  {
    to: "/dashboard/upload",
    icon: ArrowUpFromLine,
    labelKey: "upload-videos",
  },
  {
    to: "/dashboard/videos",
    icon: Video,
    labelKey: "my-videos",
  },
];

/** Settings group — appears under a "Settings" header. */
const SETTINGS_NAV: NavLink[] = [
  { to: "/dashboard/stream", icon: Radio, labelKey: "stream-settings" },
  { to: "/dashboard/keys", icon: Key, labelKey: "key-manager" },
  { to: "/dashboard/multistream", icon: Share2, labelKey: "multistream" },
  {
    to: "/dashboard/recommendations",
    icon: ListVideo,
    labelKey: "recommendations-to-others",
  },
  { to: "/dashboard/webhooks", icon: Webhook, labelKey: "webhooks" },
];

const DASHBOARD_NAV_OPEN_KEY = "streamplace:dashboard-nav-open";

export default function DashboardChrome() {
  const { t } = useTranslation("settings");
  const { state } = useSession();
  const { theatre } = useFullscreen();

  const [open, setOpen] = useState<boolean>(() => {
    if (typeof localStorage === "undefined") return true;
    return localStorage.getItem(DASHBOARD_NAV_OPEN_KEY) !== "false";
  });

  if (state.status !== "authenticated") {
    return (
      <div className="flex h-full items-center justify-center text-sm text-(--color-fg-muted)">
        {t("login-required", {
          defaultValue: "Please log in to access the dashboard.",
        })}
      </div>
    );
  }

  const user = state.session.did;

  return (
    <LivestreamProvider user={user}>
      {(store) => (
        <DashboardStoreContext.Provider value={store}>
          <DashboardMetricsProvider>
            <SidebarProvider
              className="h-svh"
              open={theatre ? false : open}
              onOpenChange={(o) => {
                setOpen(o);
                if (typeof localStorage !== "undefined") {
                  localStorage.setItem(DASHBOARD_NAV_OPEN_KEY, String(o));
                }
              }}
            >
              {!theatre && <DashboardSidebar />}
              <SidebarInset>
                {!theatre && (
                  <header className="bg-sidebar z-99 flex h-12 items-center gap-2 border-b border-(--color-border) px-4">
                    <SidebarTrigger className="-ml-1" />
                    <h1 className="font-display text-lg font-semibold">
                      {t("nav-dashboard", {
                        defaultValue: "Creator Dashboard",
                      })}
                    </h1>
                  </header>
                )}
                <div className="mx-auto flex min-h-0 w-full flex-1 flex-col">
                  <Outlet />
                </div>
              </SidebarInset>
            </SidebarProvider>
          </DashboardMetricsProvider>
        </DashboardStoreContext.Provider>
      )}
    </LivestreamProvider>
  );
}

function DashboardSidebar() {
  const { t } = useTranslation("settings");
  const { state } = useSession();
  if (state.status !== "authenticated") {
    return null;
  }
  const user = state.session.did;

  return (
    <Sidebar side="left" collapsible="icon" variant="sidebar">
      <SidebarContent>
        {/* Back to Streamplace */}
        <SidebarGroup>
          <SidebarGroupContent>
            <SidebarMenu>
              <SidebarMenuItem>
                <Link to={"/$user"} params={{ user }} className="w-full">
                  <SidebarMenuButton>
                    <ArrowLeft />
                    <span>Back to Streamplace</span>
                  </SidebarMenuButton>
                </Link>
              </SidebarMenuItem>
            </SidebarMenu>
          </SidebarGroupContent>
        </SidebarGroup>

        {/* Main nav: Control Panel + Stream Settings */}
        <SidebarGroup>
          <SidebarGroupContent>
            <SidebarMenu>
              {MAIN_NAV.map((item) => {
                const Icon = item.icon;
                return (
                  <SidebarMenuItem key={item.to}>
                    <Link to={item.to} className="w-full">
                      <SidebarMenuButton tooltip={t(item.labelKey)}>
                        <Icon />
                        <span>{t(item.labelKey)}</span>
                      </SidebarMenuButton>
                    </Link>
                  </SidebarMenuItem>
                );
              })}
            </SidebarMenu>
          </SidebarGroupContent>
        </SidebarGroup>

        {/* Settings group: stream keys, multistream, recommendations, webhooks */}
        <SidebarGroup>
          <SidebarGroupLabel>
            {t("settings-group", { defaultValue: "Settings" })}
          </SidebarGroupLabel>
          <SidebarGroupContent>
            <SidebarMenu>
              {SETTINGS_NAV.map((item) => {
                const Icon = item.icon;
                return (
                  <SidebarMenuItem key={item.to}>
                    <Link to={item.to} className="w-full">
                      <SidebarMenuButton tooltip={t(item.labelKey)}>
                        <Icon />
                        <span>{t(item.labelKey)}</span>
                      </SidebarMenuButton>
                    </Link>
                  </SidebarMenuItem>
                );
              })}
            </SidebarMenu>
          </SidebarGroupContent>
        </SidebarGroup>
      </SidebarContent>
    </Sidebar>
  );
}
