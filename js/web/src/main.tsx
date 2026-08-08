import * as Sentry from "@sentry/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { RouterProvider, createRouter } from "@tanstack/react-router";
import { StrictMode, useEffect } from "react";
import { createRoot } from "react-dom/client";
import { LoginModal } from "./components/auth/login-modal";
import { PdsHostSelectorModal } from "./components/auth/pds-host-selector-modal";
import { Toaster } from "./components/ui/sonner";
import { syncThemeClass } from "./hooks/use-color-scheme";
import "./lib/i18n";
import BlueskyProvider from "./lib/providers/bluesky-provider";
import StreamplaceProvider from "./lib/providers/streamplace-provider";
import { SessionProvider, useSession } from "./lib/session";
import { useStore } from "./lib/store";
import { routeTree } from "./routeTree.gen";
import "./styles.css";

// Initialize Sentry if DSN is provided.
const SENTRY_DSN = import.meta.env["VITE_SENTRY_DSN"] as string | undefined;
if (SENTRY_DSN) {
  Sentry.init({
    dsn: SENTRY_DSN,
    environment: import.meta.env.MODE,
    tracesSampleRate: 0.1,
  });
}

// Sync theme class before first paint to avoid flash.
syncThemeClass();

const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      staleTime: 10_000,
      retry: 1,
      refetchOnWindowFocus: false,
    },
  },
});

const router = createRouter({
  routeTree,
  defaultPreload: "intent",
  defaultPreloadStaleTime: 0,
});

declare module "@tanstack/react-router" {
  interface Register {
    router: typeof router;
  }
}

const rootEl = document.getElementById("root");
if (!rootEl) {
  throw new Error("#root element not found in index.html");
}

// Top-level modals + auth wiring. The login modal handles the
// handle-entry + sign-in flow; clicking "Sign Up" closes it and
// opens the PDS host selector, which on submit hands the chosen
// PDS to signInPopup (the PDS URL goes through as the "handle"; the
// OAuth client treats it as the identity server for the
// round-trip, and the PDS handles account creation for new users).
function AuthShell() {
  const showPdsModal = useStore((s) => s.showPdsModal);
  const closePdsModal = useStore((s) => s.closePdsModal);
  const { signIn } = useSession();

  // Register the push service worker early (on mount) so pushes
  // delivered while the tab is backgrounded still surface as system
  // notifications. The actual subscription is deferred to the
  // notifications settings toggle.
  useEffect(() => {
    void useStore.getState().initPushNotifications();
  }, []);

  return (
    <>
      <RouterProvider router={router} />
      <LoginModal />
      <PdsHostSelectorModal
        open={showPdsModal}
        onOpenChange={(open) => {
          if (!open) closePdsModal();
        }}
        onSubmit={(pdsHost) => {
          closePdsModal();
          // The OAuth client accepts a PDS URL in place of a handle
          // here; the PDS handles the auth round-trip and account
          // creation for new users.
          void signIn(pdsHost, "popup");
        }}
      />
    </>
  );
}

createRoot(rootEl).render(
  <StrictMode>
    <QueryClientProvider client={queryClient}>
      <StreamplaceProvider>
        <BlueskyProvider>
          <SessionProvider>
            <AuthShell />
          </SessionProvider>
        </BlueskyProvider>
      </StreamplaceProvider>
    </QueryClientProvider>
    <Toaster />
  </StrictMode>,
);
