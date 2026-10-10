import type { ComponentType } from "react";
import StatusOverlay from "./status-overlay";

/** Everything the overlay shell hands a widget. */
export interface OverlayProps {
  /** The streamer the overlay follows, from ?user=. */
  user: string;
  /** The build (`git describe`) serving this overlay, once the node says. */
  version: string | null;
  /** Query options from the overlay URL, including the shell's own keys
   *  (?user=, ?versionPollMs=). Widgets read their own options by name. */
  params: Record<string, string>;
}

/**
 * Widgets that can be embedded as an OBS browser source at /overlay/<name>.
 * The shell (src/screens/overlay.tsx) owns everything they have in common —
 * one upstream websocket per overlay, the version-manifest reload, a
 * transparent root, the ?user= identity — so a widget only draws its own
 * content and reads the live stream from the context the shell provides.
 */
export const OVERLAYS: Record<string, ComponentType<OverlayProps>> = {
  status: StatusOverlay,
};
