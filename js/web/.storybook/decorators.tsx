// Shared Storybook decorators that provide the context providers
// the web app's components expect: i18n, TanStack Router, Zustand store,
// and FullscreenProvider. Import the ones you need per story file.
import { FullscreenProvider } from "@/contexts/fullscreen-context";
import {
  createMemoryHistory,
  createRootRoute,
  createRoute,
  createRouter,
  RouterContextProvider,
} from "@tanstack/react-router";
import type { ReactNode } from "react";

// Minimal router: the /$user route exists so <Link> components
// targeting /$user don't throw. We use RouterContextProvider (not
// RouterProvider) because it puts the router in context for <Link>
// without rendering the route tree, letting story content pass through.
const rootRoute = createRootRoute({
  component: () => null,
});
const userRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: "/$user",
  component: () => null,
});
const routeTree = rootRoute.addChildren([userRoute]);
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
    <RouterContextProvider router={router}>
      <FullscreenProvider>{children}</FullscreenProvider>
    </RouterContextProvider>
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
