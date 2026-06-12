import { useCallback, useRef } from "react";

interface ResizeHandleProps {
  direction: "horizontal" | "vertical";
  /** Called with the delta as a fraction of the parent's size. */
  onResize: (delta: number) => void;
}

/**
 * Draggable divider between two split children.
 * Adjusts the ratio between adjacent children when dragged.
 */
export function ResizeHandle({ direction, onResize }: ResizeHandleProps) {
  const startRef = useRef<{ pos: number; size: number } | null>(null);
  const containerRef = useRef<HTMLDivElement>(null);

  const handlePointerDown = useCallback(
    (e: React.PointerEvent) => {
      e.preventDefault();
      e.stopPropagation();

      const el = containerRef.current;
      if (!el) return;
      const parent = el.parentElement;
      if (!parent) return;

      const isHorizontal = direction === "horizontal";
      const pos = isHorizontal ? e.clientX : e.clientY;
      const size = isHorizontal ? parent.clientWidth : parent.clientHeight;
      startRef.current = { pos, size };

      const handlePointerMove = (ev: PointerEvent) => {
        if (!startRef.current) return;
        const currentPos = isHorizontal ? ev.clientX : ev.clientY;
        const delta =
          (currentPos - startRef.current.pos) / startRef.current.size;
        onResize(delta);
      };

      const handlePointerUp = () => {
        startRef.current = null;
        document.removeEventListener("pointermove", handlePointerMove);
        document.removeEventListener("pointerup", handlePointerUp);
        document.body.style.cursor = "";
        document.body.style.userSelect = "";
      };

      document.addEventListener("pointermove", handlePointerMove);
      document.addEventListener("pointerup", handlePointerUp);
      document.body.style.cursor = isHorizontal ? "col-resize" : "row-resize";
      document.body.style.userSelect = "none";
    },
    [direction, onResize],
  );

  const isHorizontal = direction === "horizontal";

  return (
    <div
      ref={containerRef}
      role="separator"
      aria-orientation={isHorizontal ? "vertical" : "horizontal"}
      onPointerDown={handlePointerDown}
      className={`group z-10 flex-shrink-0 ${isHorizontal ? "w-1.5 cursor-col-resize" : "h-1.5 cursor-row-resize"} `}
    >
      <div
        className={`bg-transparent transition-colors group-hover:bg-[var(--color-accent)]/40 group-active:bg-[var(--color-accent)]/60 ${isHorizontal ? "mx-auto h-full w-0.5" : "my-auto h-0.5 w-full"} `}
      />
    </div>
  );
}
