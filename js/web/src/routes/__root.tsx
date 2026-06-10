import Header from "@/components/header";
import SidebarComponent from "@/components/sidebar";
import { TooltipProvider } from "@/components/ui/tooltip";
import { FullscreenProvider } from "@/contexts/fullscreen-context";
import {
  createRootRoute,
  Outlet,
  useRouterState,
} from "@tanstack/react-router";
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
        <div className="flex flex-col items-center justify-center min-h-svh px-6 text-center">
          <h1 className="text-2xl font-semibold font-display mb-2">
            {i18next.t("something-went-wrong")}
          </h1>
          <p className="text-[var(--color-fg-muted)] mb-6 max-w-md">
            {this.state.error.message || i18next.t("unexpected-error")}
          </p>
          <button
            type="button"
            onClick={() => this.setState({ error: null })}
            className="h-10 px-4 rounded-md bg-[var(--color-accent)] hover:bg-[var(--color-accent-hover)] text-[var(--color-accent-fg)] font-medium transition-colors"
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
    <div className="flex items-center justify-center min-h-svh">
      <div className="w-6 h-6 border-2 border-[var(--color-border)] border-t-[var(--color-accent)] rounded-full animate-spin" />
    </div>
  );
}

function RootLayout() {
  const pathname = useRouterState({
    select: (s) => s.resolvedLocation?.pathname ?? "",
  });
  const isPopout = POPOUT_PREFIXES.some((p) => pathname.startsWith(p));

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

  // Popout routes get no chrome — just providers and the outlet.
  if (isPopout) {
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
        </FullscreenProvider>
      </TooltipProvider>
    </ErrorBoundary>
  );
}
