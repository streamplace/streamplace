import type { ReactNode } from "react";

/**
 * Right-click brand menu (Copy Logo/Wordmark as SVG, Brand Guidelines).
 * Native platforms have no right-click affordance, so this is a passthrough;
 * the real implementation lives in logo-brand-menu.web.tsx.
 */
export function LogoBrandMenu({ children }: { children: ReactNode }) {
  return <>{children}</>;
}
