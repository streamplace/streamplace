import { useEffect, useState } from "react";
import { Platform } from "react-native";

/** How often an overlay asks the node whether it has been redeployed. */
export const OVERLAY_VERSION_POLL_MS = 30_000;

/**
 * Overlays (/overlay/<name>) are OBS browser sources: an operator leaves one
 * open for hours, so a deploy has to reach it without them restarting the
 * source. Poll the serving node's build manifest (GET /api/version) and reload
 * the page when it differs from the manifest this page booted against — the
 * same trick as useFrontDoorReload, keyed on the build instead of branding.
 * The manifest also names the build being served, which the overlay displays
 * so that "which deploy is this?" is answerable at a glance.
 *
 * The manifest comes from the page's own origin, not the configured API node:
 * this page's *code* is whatever that origin served, and that is what a reload
 * would replace. Web only: a native build is replaced by OTA, not by its node,
 * and has no page to reload. A failed poll is ignored until the next tick, so
 * a node hiccup reloads nobody.
 */
export function useOverlayVersion(
  pollMs: number = OVERLAY_VERSION_POLL_MS,
): string | null {
  const [version, setVersion] = useState<string | null>(null);
  const period =
    Number.isFinite(pollMs) && pollMs > 0 ? pollMs : OVERLAY_VERSION_POLL_MS;

  useEffect(() => {
    if (Platform.OS !== "web") return;
    const url = window.location.origin;
    let booted: string | null = null;
    let stopped = false;
    const check = async () => {
      let text: string;
      try {
        const res = await fetch(`${url}/api/version`, { cache: "no-store" });
        if (!res.ok) return;
        text = await res.text();
      } catch {
        return;
      }
      if (stopped) return;
      if (booted === null) {
        booted = text;
        try {
          const manifest = JSON.parse(text);
          setVersion(
            typeof manifest?.version === "string" ? manifest.version : null,
          );
        } catch {
          setVersion(null);
        }
        return;
      }
      // Any difference means a different build is now serving this page's
      // URL, and the code in memory is the one it replaced.
      if (text !== booted) window.location.reload();
    };
    void check();
    const timer = setInterval(check, period);
    return () => {
      stopped = true;
      clearInterval(timer);
    };
  }, [period]);

  return version;
}
