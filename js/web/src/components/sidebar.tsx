import { EMPTY_LOGIN_SEARCH } from "@/lib/login-search";
import { Link } from "@tanstack/react-router";
import { House, LogIn, Settings, Users, Video } from "lucide-react";
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
  return (
    <Sidebar side="left" collapsible="icon" variant="sidebar">
      <SidebarHeader className="h-10" />
      <SidebarContent>
        <SidebarGroup>
          <SidebarMenu>
            <SidebarMenuItem>
              <Link to="/" className="w-full">
                <SidebarMenuButton tooltip="Home" isActive>
                  <House />
                  <span>Home</span>
                </SidebarMenuButton>
              </Link>
            </SidebarMenuItem>
            <SidebarMenuItem>
              <Link to="/videos" className="w-full">
                <SidebarMenuButton tooltip="Videos">
                  <Video />
                  <span>Videos</span>
                </SidebarMenuButton>
              </Link>
            </SidebarMenuItem>
            <SidebarMenuItem>
              <SidebarMenuButton tooltip="Following" disabled>
                <Users />
                <span>Following</span>
              </SidebarMenuButton>
            </SidebarMenuItem>
          </SidebarMenu>
        </SidebarGroup>

        <SidebarSeparator />

        <SidebarGroup>
          <SidebarGroupLabel>Account</SidebarGroupLabel>
          <SidebarMenu>
            <SidebarMenuItem>
              <Link to="/login" search={EMPTY_LOGIN_SEARCH} className="w-full">
                <SidebarMenuButton tooltip="Log in">
                  <LogIn />
                  <span>Log in</span>
                </SidebarMenuButton>
              </Link>
            </SidebarMenuItem>
            <SidebarMenuItem>
              <Link to="/settings" className="w-full">
                <SidebarMenuButton tooltip="Settings">
                  <Settings />
                  <span>Settings</span>
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
