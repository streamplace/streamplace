export type BufferingMediaEvent =
  | "loadstart"
  | "waiting"
  | "stalled"
  | "seeking"
  | "canplay"
  | "playing"
  | "seeked"
  | "pause"
  | "ended"
  | "error";

export function getBufferingState(event: BufferingMediaEvent): boolean {
  return event === "loadstart" || event === "waiting" || event === "seeking";
}

export function shouldShowBufferingIndicator({
  active,
  buffering,
  bigPlay,
  hasError,
}: {
  active: boolean;
  buffering: boolean;
  bigPlay: boolean;
  hasError: boolean;
}): boolean {
  return active && buffering && !bigPlay && !hasError;
}

export function getBufferingOverlayPresentation(visible: boolean): {
  ariaHidden: boolean;
  opacityClass: "opacity-0" | "opacity-100";
} {
  return {
    ariaHidden: !visible,
    opacityClass: visible ? "opacity-100" : "opacity-0",
  };
}
