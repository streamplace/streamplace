import Header from "@/components/header";
import SidebarComponent from "@/components/sidebar";
import { TooltipProvider } from "@/components/ui/tooltip";
import {
  FullscreenProvider,
  useFullscreen,
} from "@/contexts/fullscreen-context";
import {
  createRootRoute,
  Outlet,
  useRouterState,
} from "@tanstack/react-router";
import { Loader } from "lucide-react";
import { Component, type ReactNode, useEffect, useState } from "react";
import { SidebarInset, SidebarProvider } from "../components/ui/sidebar";
import { getStoredPreference, syncThemeClass } from "../hooks/use-color-scheme";
import i18next from "../lib/i18n";

/** Routes that should render without sidebar/header chrome. */
const POPOUT_PREFIXES = ["/chat-popout/", "/embed/"];

/** Routes that own their own chrome via a layout route
 *  (see routes/dashboard/route.tsx). The root only needs to know about
 *  these to decide whether to wrap with ChromeLayout or render the
 *  Outlet bare. The dashboard's provider tree is mounted by the layout
 *  route itself, not by this check, so a stale read here is just a
 *  visual flash; not a render race. */
const DASHBOARD_PREFIX = "/dashboard";

export const Route = createRootRoute({
  component: RootLayout,
  pendingComponent: RouteLoadingSkeleton,
});

interface ErrorBoundaryState {
  error: Error | null;
}

class ErrorBoundary extends Component<
  { children: ReactNode },
  ErrorBoundaryState
> {
  state: ErrorBoundaryState = { error: null };

  static getDerivedStateFromError(error: Error) {
    return { error };
  }

  componentDidCatch(error: Error, info: React.ErrorInfo) {
    console.error("Unhandled render error:", error, info);
  }

  render() {
    if (this.state.error) {
      return (
        <div className="flex min-h-svh flex-col items-center justify-center px-6 text-center">
          <h1 className="font-display mb-2 text-2xl font-semibold">
            {i18next.t("something-went-wrong")}
          </h1>
          <p className="mb-6 max-w-md text-(--color-fg-muted)">
            {this.state.error.message || i18next.t("unexpected-error")}
          </p>
          <button
            type="button"
            onClick={() => this.setState({ error: null })}
            className="h-10 rounded-md bg-(--color-accent) px-4 font-medium text-(--color-accent-fg) transition-colors hover:bg-(--color-accent-hover)"
          >
            {i18next.t("try-again")}
          </button>
        </div>
      );
    }
    return this.props.children;
  }
}

function RouteLoadingSkeleton() {
  return (
    <div className="flex min-h-svh items-center justify-center">
      <Loader className="animate-spin text-(--color-fg-muted)" />
    </div>
  );
}

function RootLayout() {
  // Popout routes (chat popout, embeds) skip the regular chrome and
  // render their own minimal layout. Dashboard routes also skip the
  // regular chrome; they own their own via routes/dashboard/route.tsx.
  // The dashboard layout route mounts DashboardChrome (with its
  // DashboardStoreContext etc.) before its children, so the chrome
  // decision here is purely visual; a stale read just causes a brief
  // flash, not a render race.
  const pathname = useRouterState({
    select: (s) => s.resolvedLocation?.pathname ?? "",
  });

  const browserPathname =
    typeof window !== "undefined" ? window.location.pathname : "";

  const actualPathname = pathname || browserPathname;

  // pause until we can get a valid pathname, to avoid rendering the wrong chrome on initial load
  if (!actualPathname) {
    return <RouteLoadingSkeleton />;
  }

  if (POPOUT_PREFIXES.some((p) => actualPathname.startsWith(p))) {
    return (
      <ErrorBoundary>
        <TooltipProvider>
          <Outlet />
        </TooltipProvider>
      </ErrorBoundary>
    );
  }

  const isDashboard =
    actualPathname === DASHBOARD_PREFIX ||
    actualPathname.startsWith(`${DASHBOARD_PREFIX}/`);

  if (isDashboard) {
    return (
      <ErrorBoundary>
        <TooltipProvider>
          <FullscreenProvider>
            <Outlet />
          </FullscreenProvider>
        </TooltipProvider>
      </ErrorBoundary>
    );
  }

  return (
    <ErrorBoundary>
      <TooltipProvider>
        <FullscreenProvider>
          <ChromeLayout />
        </FullscreenProvider>
      </TooltipProvider>
    </ErrorBoundary>
  );
}

/**
 * Renders sidebar + header chrome. The Outlet is always mounted so that
 * route-level state (player, chat WebSocket, etc.) persists when theatre
 * mode toggles. Only the chrome siblings are conditionally rendered.
 */
function ChromeLayout() {
  const { theatre } = useFullscreen();

  const [open, setOpen] = useState(() => {
    if (typeof localStorage === "undefined") return true;
    return localStorage.getItem("streamplace:nav-open") !== "false";
  });

  // Re-sync theme when system preference changes (only matters if pref is "system").
  useEffect(() => {
    const mql = window.matchMedia("(prefers-color-scheme: dark)");
    const onChange = () => {
      if (getStoredPreference() === "system") {
        syncThemeClass();
      }
    };
    mql.addEventListener("change", onChange);
    return () => mql.removeEventListener("change", onChange);
  }, []);

  return (
    <SidebarProvider
      open={theatre ? false : open}
      onOpenChange={(o) => {
        setOpen(o);
        localStorage.setItem("streamplace:nav-open", String(o));
      }}
      className="h-svh"
    >
      {!theatre && <SidebarComponent />}
      <SidebarInset>
        {!theatre && <Header />}
        <div className="mx-auto flex min-h-0 w-full flex-1 flex-col">
          <Outlet />
        </div>
      </SidebarInset>
    </SidebarProvider>
  );
}
