import type { LivestreamStore } from "@streamplace/core";
import type { ComponentType } from "react";
import { StatusOverlay } from "./status-overlay";

/** Everything the overlay route hands a widget. */
export interface OverlayProps {
  /** The streamer the overlay follows, from ?user=. */
  user: string;
  /** The upstream websocket-backed store for that streamer. */
  store: LivestreamStore;
  /** The build (`git describe`) serving this overlay, once the node says. */
  version: string | null;
  /** Query options from the overlay URL, including the shell's own keys
   *  (?user=, ?versionPollMs=). Widgets read their own options by name. */
  params: Record<string, string>;
}

/**
 * Widgets that can be embedded as an OBS browser source at /overlay/<name>.
 * The route (src/routes/overlay/$name.tsx) owns everything they have in
 * common — the upstream websocket, the version-manifest reload, a transparent
 * root, the ?user= identity — so a widget only draws its own content.
 */
export const OVERLAYS: Record<string, ComponentType<OverlayProps>> = {
  status: StatusOverlay,
};

/**
 * The widget registered under `name`, or undefined. The name comes from the
 * URL, so a plain lookup is not enough: `__proto__` and friends resolve to
 * something inherited from Object.prototype.
 */
export function overlayFor(
  name: string,
): ComponentType<OverlayProps> | undefined {
  return Object.prototype.hasOwnProperty.call(OVERLAYS, name)
    ? OVERLAYS[name]
    : undefined;
}
