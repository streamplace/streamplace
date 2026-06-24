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

export const Route = createRootRoute({
  component: RootLayout,
  pendingComponent: RouteLoadingSkeleton,
});

// --- Error boundary ---

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

// --- Root layout ---

function RouteLoadingSkeleton() {
  return (
    <div className="flex min-h-svh items-center justify-center">
      <Loader className="animate-spin text-(--color-fg-muted)" />
    </div>
  );
}

function RootLayout() {
  // Popout routes (chat popout, embeds) skip the regular chrome and
  // render their own minimal layout. Everything else gets the full
  // provider tree; the chrome decision (regular vs dashboard) is now
  // owned by route layouts — see routes/dashboard/route.tsx for the
  // dashboard chrome layout, and the non-dashboard routes get the
  // regular chrome via ChromeLayout below. The root no longer switches
  // chrome based on pathname, which closed a render race where the
  // dashboard route was mounted without its DashboardStoreContext.
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
