import { Link, useMatchRoute } from "@tanstack/react-router";
import {
  House,
  LayoutDashboard,
  LogIn,
  Settings,
  Users,
  Video,
} from "lucide-react";
import { useTranslation } from "react-i18next";
import { useSession } from "../lib/session";
import { useStore } from "../lib/store";
import {
  Sidebar,
  SidebarContent,
  SidebarFooter,
  SidebarGroup,
  SidebarGroupLabel,
  SidebarHeader,
  SidebarMenu,
  SidebarMenuButton,
  SidebarMenuItem,
  SidebarSeparator,
} from "./ui/sidebar";

export default function SidebarComponent() {
  const { t } = useTranslation("common");
  const { state } = useSession();
  const isAuthenticated = state.status === "authenticated";
  const matchRoute = useMatchRoute();

  // Active state follows the current route. Home matches exactly (the
  // root route is an ancestor of every page); the others match their
  // sub-routes too (e.g. /settings/account highlights Settings).
  const isHomeActive = matchRoute({ to: "/", fuzzy: false }) !== false;
  const isVideosActive = matchRoute({ to: "/videos", fuzzy: false }) !== false;
  const isDashboardActive =
    matchRoute({ to: "/dashboard", fuzzy: true }) !== false;
  const isSettingsActive =
    matchRoute({ to: "/settings", fuzzy: true }) !== false;

  return (
    <Sidebar side="left" collapsible="icon" variant="sidebar">
      <SidebarHeader className="h-10" />
      <SidebarContent>
        <SidebarGroup>
          <SidebarMenu>
            <SidebarMenuItem>
              <Link to="/" className="w-full">
                <SidebarMenuButton
                  tooltip={t("nav-home")}
                  isActive={isHomeActive}
                >
                  <House />
                  <span>{t("nav-home")}</span>
                </SidebarMenuButton>
              </Link>
            </SidebarMenuItem>
            <SidebarMenuItem>
              <Link to="/videos" className="w-full">
                <SidebarMenuButton
                  tooltip={t("nav-videos")}
                  isActive={isVideosActive}
                >
                  <Video />
                  <span>{t("nav-videos")}</span>
                </SidebarMenuButton>
              </Link>
            </SidebarMenuItem>
            <SidebarMenuItem>
              <SidebarMenuButton tooltip={t("nav-following")} disabled>
                <Users />
                <span>{t("nav-following")}</span>
              </SidebarMenuButton>
            </SidebarMenuItem>
            {isAuthenticated && (
              <SidebarMenuItem>
                <Link to="/dashboard" className="w-full">
                  <SidebarMenuButton
                    tooltip={t("nav-dashboard", { defaultValue: "Dashboard" })}
                    isActive={isDashboardActive}
                  >
                    <LayoutDashboard />
                    <span>
                      {t("nav-dashboard", { defaultValue: "Dashboard" })}
                    </span>
                  </SidebarMenuButton>
                </Link>
              </SidebarMenuItem>
            )}
          </SidebarMenu>
        </SidebarGroup>

        <SidebarSeparator />

        <SidebarGroup>
          <SidebarGroupLabel>{t("nav-account")}</SidebarGroupLabel>
          <SidebarMenu>
            <SidebarMenuItem>
              <SidebarMenuButton
                tooltip={t("log-in")}
                onClick={() => useStore.getState().openLoginModal()}
              >
                <LogIn />
                <span>{t("log-in")}</span>
              </SidebarMenuButton>
            </SidebarMenuItem>
            <SidebarMenuItem>
              <Link to="/settings" className="w-full">
                <SidebarMenuButton
                  tooltip={t("nav-settings")}
                  isActive={isSettingsActive}
                >
                  <Settings />
                  <span>{t("nav-settings")}</span>
                </SidebarMenuButton>
              </Link>
            </SidebarMenuItem>
          </SidebarMenu>
        </SidebarGroup>
      </SidebarContent>

      <SidebarFooter></SidebarFooter>
    </Sidebar>
  );
}
