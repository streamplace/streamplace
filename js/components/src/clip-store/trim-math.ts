// Pure trim-window math for the clip editor. No RN imports — this is the unit
// test surface (see trim-math.test.ts). All times are in milliseconds.
//
// The functions are marked as reanimated worklets so the TrimTimeline gestures
// can call them on the UI thread; on the JS thread the directive is inert.

export const MIN_CLIP_MS = 5000;

export type DragZone = "left" | "body" | "right" | "none";

export function msToPx(
  ms: number,
  durationMs: number,
  trackWidth: number,
): number {
  "worklet";
  if (durationMs <= 0 || trackWidth <= 0) return 0;
  return (ms / durationMs) * trackWidth;
}

export function pxToMs(
  px: number,
  durationMs: number,
  trackWidth: number,
): number {
  "worklet";
  if (durationMs <= 0 || trackWidth <= 0) return 0;
  return (px / trackWidth) * durationMs;
}

// Clamp a [start, end] window to the track and enforce the minimum window
// size. When the window would be smaller than minMs we push the far boundary
// out instead of shrinking the selection below the minimum. If the track
// itself is shorter than minMs, the whole track is the window.
export function clampWindow(
  start: number,
  end: number,
  durationMs: number,
  minMs = MIN_CLIP_MS,
): { start: number; end: number } {
  "worklet";
  if (durationMs <= 0) return { start: 0, end: 0 };
  let s = Math.min(Math.max(0, start), durationMs);
  let e = Math.min(Math.max(0, end), durationMs);
  if (e < s) {
    const tmp = s;
    s = e;
    e = tmp;
  }
  const min = Math.min(minMs, durationMs);
  if (e - s < min) {
    const need = min - (e - s);
    const extendEnd = Math.min(need, durationMs - e);
    e += extendEnd;
    s -= need - extendEnd;
  }
  return { start: s, end: e };
}

// Slide the whole window by deltaMs, clamped to the track bounds and keeping
// its size (the window is already >= minMs from clampWindow).
export function moveWindow(
  start: number,
  end: number,
  deltaMs: number,
  durationMs: number,
  _minMs = MIN_CLIP_MS,
): { start: number; end: number } {
  "worklet";
  if (durationMs <= 0) return { start: 0, end: 0 };
  const w = Math.max(0, end - start);
  let s = start + deltaMs;
  let e = s + w;
  if (s < 0) {
    s = 0;
    e = Math.min(w, durationMs);
  }
  if (e > durationMs) {
    e = durationMs;
    s = Math.max(0, durationMs - w);
  }
  return { start: s, end: e };
}

// Resize the window by dragging one edge. The dragged edge follows newEdgeMs,
// clamped so the window never shrinks below minMs (the dragged edge stops,
// the opposite edge stays fixed).
export function resizeWindow(
  edge: "left" | "right",
  start: number,
  end: number,
  newEdgeMs: number,
  durationMs: number,
  minMs = MIN_CLIP_MS,
): { start: number; end: number } {
  "worklet";
  if (durationMs <= 0) return { start: 0, end: 0 };
  const min = Math.min(minMs, durationMs);
  const clamped = Math.min(Math.max(0, newEdgeMs), durationMs);
  if (edge === "left") {
    const maxLeft = Math.max(0, end - min);
    return { start: Math.min(clamped, maxLeft), end };
  }
  const minRight = start + min;
  return { start, end: Math.max(clamped, minRight) };
}

// Hit-test a pointer x (relative to the track) against the selection window:
// the handle zones at either edge, the window body, or empty track.
export function dragZone(
  x: number,
  windowLeft: number,
  windowRight: number,
  handleWidth = 12,
): DragZone {
  "worklet";
  if (x < windowLeft - handleWidth || x > windowRight + handleWidth)
    return "none";
  if (x <= windowLeft + handleWidth) return "left";
  if (x >= windowRight - handleWidth) return "right";
  return "body";
}
