import { EMPTY_LOGIN_SEARCH } from "@/lib/login-search";
import { Link } from "@tanstack/react-router";
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

  return (
    <Sidebar side="left" collapsible="icon" variant="sidebar">
      <SidebarHeader className="h-10" />
      <SidebarContent>
        <SidebarGroup>
          <SidebarMenu>
            <SidebarMenuItem>
              <Link to="/" className="w-full">
                <SidebarMenuButton tooltip={t("nav-home")} isActive>
                  <House />
                  <span>{t("nav-home")}</span>
                </SidebarMenuButton>
              </Link>
            </SidebarMenuItem>
            <SidebarMenuItem>
              <Link to="/videos" className="w-full">
                <SidebarMenuButton tooltip={t("nav-videos")}>
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
              <Link to="/login" search={EMPTY_LOGIN_SEARCH} className="w-full">
                <SidebarMenuButton tooltip={t("log-in")}>
                  <LogIn />
                  <span>{t("log-in")}</span>
                </SidebarMenuButton>
              </Link>
            </SidebarMenuItem>
            <SidebarMenuItem>
              <Link to="/settings" className="w-full">
                <SidebarMenuButton tooltip={t("nav-settings")}>
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
