import { RouterProvider, createRouter } from "@tanstack/react-router";
import { StrictMode } from "react";
import { createRoot } from "react-dom/client";
import { Toaster } from "./components/ui/sonner";
import BlueskyProvider from "./lib/providers/bluesky-provider";
import StreamplaceProvider from "./lib/providers/streamplace-provider";
import { SessionProvider } from "./lib/session";
import { routeTree } from "./routeTree.gen";
import "./styles.css";

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
    <StreamplaceProvider>
      <BlueskyProvider>
        <SessionProvider>
          <RouterProvider router={router} />
        </SessionProvider>
      </BlueskyProvider>
    </StreamplaceProvider>
    <Toaster />
  </StrictMode>,
);
