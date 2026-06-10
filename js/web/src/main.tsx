import * as Sentry from "@sentry/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { RouterProvider, createRouter } from "@tanstack/react-router";
import { StrictMode } from "react";
import { createRoot } from "react-dom/client";
import { Toaster } from "./components/ui/sonner";
import { syncThemeClass } from "./hooks/use-color-scheme";
import "./lib/i18n";
import BlueskyProvider from "./lib/providers/bluesky-provider";
import StreamplaceProvider from "./lib/providers/streamplace-provider";
import { SessionProvider } from "./lib/session";
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

createRoot(rootEl).render(
  <StrictMode>
    <QueryClientProvider client={queryClient}>
      <StreamplaceProvider>
        <BlueskyProvider>
          <SessionProvider>
            <RouterProvider router={router} />
          </SessionProvider>
        </BlueskyProvider>
      </StreamplaceProvider>
    </QueryClientProvider>
    <Toaster />
  </StrictMode>,
);
