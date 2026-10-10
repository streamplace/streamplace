// The generic overlay host: /overlay/<name>, designed to be embedded in OBS
// as a browser source. It owns what every widget needs — the streamer
// identity from ?user=, the upstream websocket, the version-manifest reload,
// a transparent full-bleed root — and renders the widget registered under
// <name> (see @/components/overlay/registry).
import { OverlayDiagnostic } from "@/components/overlay/overlay-diagnostic";
import { overlayFor } from "@/components/overlay/registry";
import { useLivestreamStore } from "@/hooks/use-livestream-store";
import { useOverlayVersion } from "@/hooks/use-overlay-version";
import { createFileRoute } from "@tanstack/react-router";
import { useEffect } from "react";

export const Route = createFileRoute("/overlay/$name")({
  // Widgets read their own options (and the shell reads ?user= and
  // ?versionPollMs), so keep every query parameter rather than a fixed set.
  // TanStack's default parser turns numeric-looking values into numbers (and
  // "true"/"false" into booleans); widget options are strings, so put them
  // back rather than dropping them.
  validateSearch: (search: Record<string, unknown>): Record<string, string> => {
    const params: Record<string, string> = {};
    for (const [key, value] of Object.entries(search)) {
      if (value === null || typeof value === "object") continue;
      params[key] = String(value);
    }
    return params;
  },
  component: OverlayHost,
});

function OverlayHost() {
  const { name } = Route.useParams();
  const params = Route.useSearch();
  const user = params.user ?? "";

  // An overlay is composited over the operator's scene, so the page itself
  // must not paint a background. The web app paints one on <html> (see
  // styles.css) and OBS's own default custom CSS only clears <body>, so clear
  // both while an overlay is on screen.
  useEffect(() => {
    const html = document.documentElement;
    const body = document.body;
    const backgrounds = [
      html.style.backgroundColor,
      body.style.backgroundColor,
    ];
    html.style.backgroundColor = "transparent";
    body.style.backgroundColor = "transparent";
    return () => {
      html.style.backgroundColor = backgrounds[0];
      body.style.backgroundColor = backgrounds[1];
    };
  }, []);

  // Reloading on a new deployment is the shell's job, not the widget's; the
  // widget only displays the build it is showing.
  const version = useOverlayVersion(Number(params.versionPollMs));
  const { store } = useLivestreamStore(user, user !== "");

  const Overlay = overlayFor(name);
  if (!Overlay) {
    return (
      <OverlayDiagnostic
        title={`No overlay named "${name}"`}
        detail="Check the name in the browser source URL."
      />
    );
  }

  if (!user) {
    return (
      <OverlayDiagnostic
        title="This overlay needs a streamer"
        detail={`Add ?user=<your handle> to the URL, e.g. /overlay/${name}?user=your.handle`}
      />
    );
  }

  if (!store) {
    return <div className="h-screen w-screen bg-transparent" />;
  }

  return (
    <Overlay user={user} store={store} version={version} params={params} />
  );
}
