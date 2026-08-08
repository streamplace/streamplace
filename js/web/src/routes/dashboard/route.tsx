import DashboardChrome from "@/components/dashboard/dashboard-chrome";
import { createFileRoute } from "@tanstack/react-router";

// Layout route for everything under /dashboard. DashboardChrome renders
// its own <Outlet /> for the child route, and provides the
// DashboardStoreContext, LivestreamProvider, DashboardMetricsProvider,
// and the dashboard's own SidebarProvider.
//
// Having this as a real layout route (instead of conditionally rendering
// DashboardChrome in __root.tsx based on the current pathname) avoids a
// render race: the dashboard child route's `useDashboardStore()` call
// was throwing because the dashboard chrome wasn't always mounted at the
// time the child was rendered. With a layout route, the chrome and the
// provider tree are guaranteed to be in place before the child renders.
export const Route = createFileRoute("/dashboard")({
  component: DashboardChrome,
});
