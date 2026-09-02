import { useLivestreamStore } from "@/hooks/use-livestream-store";
import type { LivestreamStore } from "@streamplace/core";
import { createFileRoute } from "@tanstack/react-router";
import { useStore } from "zustand";

export const Route = createFileRoute("/embed/info-widget/$user")({
  component: EmbedInfoWidget,
});

function EmbedInfoWidget() {
  const { user } = Route.useParams();
  const { store, ready } = useLivestreamStore(user);

  if (!ready || !store) {
    return (
      <div className="flex h-screen w-screen items-center justify-center bg-transparent">
        <div className="h-5 w-5 animate-spin rounded-full border-2 border-white/30 border-t-white" />
      </div>
    );
  }

  return <InfoWidgetBody store={store} />;
}

function InfoWidgetBody({ store }: { store: LivestreamStore }) {
  const state = useStore(store, (s) => ({
    livestream: s.livestream,
    segment: s.segment,
    viewers: s.viewers,
    websocketConnected: s.websocketConnected,
  }));

  const videoTrack = state.segment?.video?.[0];
  const width = videoTrack?.width ?? 0;
  const height = videoTrack?.height ?? 0;

  const bitrate =
    state.segment?.size && state.segment?.duration
      ? (
          (state.segment.size * 8) /
          (state.segment.duration / 1_000_000_000) /
          1000
        ).toFixed(0)
      : "0";

  const fps = videoTrack?.framerate
    ? Math.round(videoTrack.framerate.num / videoTrack.framerate.den)
    : null;

  return (
    <div className="flex h-screen w-screen items-start justify-end bg-transparent p-4">
      <div className="min-w-[200px] space-y-2 rounded-lg bg-black/70 p-4 font-mono text-sm text-white backdrop-blur-sm">
        <div className="flex items-center gap-2">
          <div
            className={`h-2 w-2 rounded-full ${state.websocketConnected ? "bg-green-400" : "bg-red-400"}`}
          />
          <span className="text-xs tracking-wider uppercase opacity-70">
            {state.websocketConnected ? "connected" : "disconnected"}
          </span>
        </div>

        {state.viewers !== null && (
          <div className="flex justify-between">
            <span className="opacity-70">Viewers</span>
            <span>{state.viewers}</span>
          </div>
        )}

        {width > 0 && height > 0 && (
          <div className="flex justify-between">
            <span className="opacity-70">Resolution</span>
            <span>
              {width}x{height}
            </span>
          </div>
        )}

        {fps !== null && (
          <div className="flex justify-between">
            <span className="opacity-70">FPS</span>
            <span>{fps}</span>
          </div>
        )}

        <div className="flex justify-between">
          <span className="opacity-70">Bitrate</span>
          <span>{bitrate} kbps</span>
        </div>
      </div>
    </div>
  );
}
