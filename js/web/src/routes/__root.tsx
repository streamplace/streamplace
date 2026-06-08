import Header from "@/components/header";
import SidebarComponent from "@/components/sidebar";
import { createRootRoute, Outlet } from "@tanstack/react-router";
import { useState } from "react";
import { SidebarInset, SidebarProvider } from "../components/ui/sidebar";

export const Route = createRootRoute({
  component: RootLayout,
});

function RootLayout() {
  const [open, setOpen] = useState(() => {
    if (typeof localStorage === "undefined") return true;
    return localStorage.getItem("streamplace:nav-open") !== "false";
  });

  return (
    <SidebarProvider
      open={open}
      onOpenChange={(o) => {
        setOpen(o);
        localStorage.setItem("streamplace:nav-open", String(o));
      }}
      className="h-svh"
    >
      <SidebarComponent />
      <SidebarInset>
        <Header />
        <div className="flex flex-1 flex-col min-h-0 mx-auto w-full">
          <Outlet />
        </div>
      </SidebarInset>
    </SidebarProvider>
  );
}
