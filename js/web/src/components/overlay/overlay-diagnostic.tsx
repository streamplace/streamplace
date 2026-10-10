import { OVERLAYS } from "./registry";

/**
 * What an operator sees in OBS when the overlay URL is wrong. An overlay is
 * addressed only by its URL, so there is no UI to complain: this card has to
 * be legible over the scene they are building, and it lists the names that
 * would work.
 */
export function OverlayDiagnostic({
  title,
  detail,
}: {
  title: string;
  detail: string;
}) {
  return (
    <div
      className="flex h-screen w-screen items-center justify-center bg-transparent p-4"
      data-testid="overlay-diagnostic"
    >
      <div className="border-border bg-background/85 flex max-w-[420px] flex-col gap-2 rounded-lg border p-4 backdrop-blur-sm">
        <p className="text-foreground text-base font-semibold">{title}</p>
        <p className="text-muted-foreground text-sm">{detail}</p>
        <p className="text-muted-foreground text-xs">
          Available overlays: {Object.keys(OVERLAYS).sort().join(", ")}
        </p>
      </div>
    </div>
  );
}
