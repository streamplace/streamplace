import { useStore } from "zustand";
import type { OverlayProps } from "./registry";

/**
 * The starter overlay: what this widget is talking to, and whether the
 * upstream websocket is up. It is the overlay an operator points OBS at while
 * they are setting a scene up, and it exercises both halves of the shell —
 * the websocket (connected / live / viewers) and the version manifest (the
 * node build it is showing).
 */
export function StatusOverlay({ user, store, version }: OverlayProps) {
  const websocketConnected = useStore(store, (s) => s.websocketConnected);
  const profile = useStore(store, (s) => s.profile);
  const livestream = useStore(store, (s) => s.livestream);
  const viewers = useStore(store, (s) => s.viewers);

  return (
    <div
      className="flex h-screen w-screen items-start justify-start bg-transparent p-4"
      data-testid="overlay-status"
    >
      <div className="border-border bg-background/85 flex min-w-[220px] flex-col gap-2 rounded-lg border p-4 backdrop-blur-sm">
        <div className="flex items-center gap-2">
          <div
            className={`h-2 w-2 rounded-full ${websocketConnected ? "bg-success" : "bg-muted-foreground"}`}
          />
          <span
            className="text-muted-foreground text-xs font-semibold tracking-wider uppercase"
            data-testid="overlay-upstream"
          >
            {websocketConnected ? "Connected" : "Connecting"}
          </span>
        </div>
        <Row
          label="Streamer"
          value={profile?.handle ?? user}
          testId="overlay-streamer"
        />
        <Row
          label="Node"
          value={version ?? "unknown"}
          testId="overlay-version"
        />
        <Row
          label="Live"
          value={livestream ? (livestream.record.title ?? "yes") : "offline"}
        />
        {viewers !== null && <Row label="Viewers" value={String(viewers)} />}
      </div>
    </div>
  );
}

function Row({
  label,
  value,
  testId,
}: {
  label: string;
  value: string;
  testId?: string;
}) {
  return (
    <div className="flex justify-between gap-4 text-sm">
      <span className="text-muted-foreground">{label}</span>
      <span
        className="text-foreground"
        style={{ flexShrink: 1 }}
        data-testid={testId}
      >
        {value}
      </span>
    </div>
  );
}
