import { LivestreamProvider } from "@streamplace/components";
import { OverlayDiagnostic } from "components/overlay/overlay-diagnostic";
import { overlayFor } from "components/overlay/registry";
import { useOverlayVersion } from "hooks/useOverlayVersion";
import { useEffect } from "react";
import { Platform, View } from "react-native";
import { useStore } from "store";

/**
 * The generic overlay host: /overlay/<name>, designed to be embedded in OBS
 * as a browser source. It owns what every widget needs — the streamer
 * identity from ?user=, the upstream websocket, the version-manifest reload,
 * a transparent full-bleed root — and renders the widget registered under
 * <name> (see components/overlay/registry.tsx).
 */
export default function OverlayScreen({ route }) {
  const setSidebarHidden = useStore((state) => state.setSidebarHidden);
  const setSidebarUnhidden = useStore((state) => state.setSidebarUnhidden);
  useEffect(() => {
    setSidebarHidden();
    return () => setSidebarUnhidden();
  }, [setSidebarHidden, setSidebarUnhidden]);

  // An overlay is composited over the operator's scene, so the page itself
  // must not paint a background. The app sets one on <html> and <body> (OBS's
  // own default custom CSS only clears <body>), so clear both while an
  // overlay is on screen.
  useEffect(() => {
    if (Platform.OS !== "web") return;
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

  const name = typeof route.params?.name === "string" ? route.params.name : "";
  // On web an overlay is a URL in someone else's page, so read the query
  // string like the other embed screens do; on native the linking config
  // lands the query in the route params.
  const params: Record<string, string> = {};
  if (Platform.OS === "web") {
    new URLSearchParams(window.location.search).forEach((value, key) => {
      params[key] = value;
    });
  } else {
    for (const [key, value] of Object.entries(route.params ?? {})) {
      if (key !== "name" && typeof value === "string") params[key] = value;
    }
  }

  // Reloading on a new deployment is the shell's job, not the widget's; the
  // widget only displays the build it is showing.
  const version = useOverlayVersion(Number(params.versionPollMs));

  const Overlay = overlayFor(name);
  if (!Overlay) {
    return (
      <OverlayDiagnostic
        title={`No overlay named "${name}"`}
        detail="Check the name in the browser source URL."
      />
    );
  }

  const user = params.user;
  if (!user) {
    return (
      <OverlayDiagnostic
        title="This overlay needs a streamer"
        detail={`Add ?user=<your handle> to the URL, e.g. /overlay/${name}?user=your.handle`}
      />
    );
  }

  return (
    <LivestreamProvider src={user}>
      {/* Absolute fill, so the widget owns the whole viewport no matter how
          the navigator lays its screens out (see the danmu OBS screen). */}
      <View
        style={{ position: "absolute", top: 0, left: 0, right: 0, bottom: 0 }}
      >
        <Overlay user={user} version={version} params={params} />
      </View>
    </LivestreamProvider>
  );
}
