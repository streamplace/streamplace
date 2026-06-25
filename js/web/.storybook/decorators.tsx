// Shared Storybook decorators that provide the context providers
// the web app's components expect: i18n, TanStack Router, Zustand store,
// and FullscreenProvider. Import the ones you need per story file.
import { FullscreenProvider } from "@/contexts/fullscreen-context";
import "@/lib/i18n";
import { useStore } from "@/lib/store";
import {
  createMemoryHistory,
  createRootRoute,
  createRoute,
  createRouter,
  RouterProvider,
} from "@tanstack/react-router";
import type { ReactNode } from "react";

// Initialise the store so useStreamplaceUrl etc. don't blow up.
// The slice reads from localStorage at init; in Storybook (jsdom)
// localStorage is available but empty, so it falls back to
// window.location.origin.
useStore.getState().initialize();

// A minimal router with a catch-all root route so <Link> works.
const rootRoute = createRootRoute({
  component: () => null,
});
const indexRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: "/$user",
  component: () => null,
});
const routeTree = rootRoute.addChildren([indexRoute]);
const router = createRouter({
  routeTree,
  history: createMemoryHistory({ initialEntries: ["/"] }),
});

declare module "@tanstack/react-router" {
  interface Register {
    router: typeof router;
  }
}

export function withProviders({ children }: { children: ReactNode }) {
  return (
    <RouterProvider router={router}>
      <FullscreenProvider>{children}</FullscreenProvider>
    </RouterProvider>
  );
}

// Convenience decorator for stories that only need FullscreenProvider
// (e.g. Player).
export function withFullscreen(Story: () => ReactNode) {
  return <FullscreenProvider>{<Story />}</FullscreenProvider>;
}

// Convenience decorator for stories that need the full provider stack
// (Router + i18n + Store + Fullscreen).
export function withAllProviders(Story: () => ReactNode) {
  return withProviders({ children: <Story /> });
}
