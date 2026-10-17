import type { LivestreamStore } from "@streamplace/core";
import { createContext, useContext } from "react";

/**
 * Context that exposes the per-user `LivestreamStore` to all dashboard
 * sub-routes. Set up by `DashboardChrome`, consumed by the metrics
 * tracker, the stream-health widget, and the control panel.
 */
export const DashboardStoreContext = createContext<LivestreamStore | null>(
  null,
);

export function useIsDashboardStoreReady(): boolean {
  const store = useContext(DashboardStoreContext);
  return !!store;
}

/**
 * Hook for consuming the dashboard's `LivestreamStore`. Must be used inside
 * `<DashboardChrome>`, which provides the context.
 */
export function useDashboardStore(): LivestreamStore {
  const store = useContext(DashboardStoreContext);
  if (!store) {
    throw new Error("useDashboardStore must be used inside <DashboardChrome>");
  }
  return store;
}
