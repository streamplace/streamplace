import { cn } from "@/lib/utils";
import type { LivestreamStore } from "@streamplace/core";
import { ChevronDown, GripVertical, X } from "lucide-react";
import { useCallback, useEffect, useRef, useState } from "react";
import type { DragState } from "./layout-renderer";
import { WIDGET_KEYS, getWidgetMeta } from "./registry";

interface WidgetLeafProps {
  widgetKey: string;
  store: LivestreamStore;
  user?: string;
  /** Called when the user picks a different widget from the dropdown. */
  onWidgetChange?: (newWidget: string) => void;
  /** Called when the user clicks the X to remove this slot from the tree. */
  onWidgetRemove?: () => void;
  /** Whether this leaf is currently being dragged. */
  isDragging?: boolean;
  /** Ref for the drag handle (dnd-kit handleRef). */
  handleRef?: React.Ref<HTMLDivElement>;
  /** Ref for the sortable element (dnd-kit ref). */
  sortableRef?: React.Ref<HTMLDivElement>;
  /**
   * When non-null, this leaf is the current drop target. Pointer position
   * is used to draw a split-preview overlay (which half the source widget
   * would land in on drop).
   */
  dropTarget?: DragState | null;
  /** Unique sortable id, used to attach a data attribute for lookup. */
  leafId?: string;
}

/**
 * Leaf chrome: title bar with widget icon, drag handle, and a dropdown
 * to swap the widget type. Wraps the actual widget component.
 */
export function WidgetLeaf({
  widgetKey,
  store,
  user,
  onWidgetChange,
  onWidgetRemove,
  isDragging,
  handleRef,
  sortableRef,
  dropTarget,
  leafId,
}: WidgetLeafProps) {
  const meta = getWidgetMeta(widgetKey);
  const [dropdownOpen, setDropdownOpen] = useState(false);
  const rootRef = useRef<HTMLDivElement | null>(null);
  const [quadrant, setQuadrant] = useState<Quadrant | null>(null);

  // Recompute which quadrant of the leaf the cursor is over whenever
  // the drop target updates. This drives the split-preview overlay.
  useEffect(() => {
    if (!dropTarget || !rootRef.current) {
      setQuadrant(null);
      return;
    }
    const rect = rootRef.current.getBoundingClientRect();
    const relX = (dropTarget.pointer.x - rect.left) / rect.width;
    const relY = (dropTarget.pointer.y - rect.top) / rect.height;
    if (relX < 0 || relX > 1 || relY < 0 || relY > 1) {
      setQuadrant(null);
      return;
    }
    const horizontalDominant = Math.abs(relX - 0.5) >= Math.abs(relY - 0.5);
    setQuadrant({
      direction: horizontalDominant ? "horizontal" : "vertical",
      first: horizontalDominant ? relX < 0.5 : relY < 0.5,
    });
  }, [dropTarget]);

  const setRefs = useCallback(
    (el: HTMLDivElement | null) => {
      rootRef.current = el;
      if (typeof sortableRef === "function") sortableRef(el);
      else if (sortableRef && "current" in sortableRef) {
        (sortableRef as React.MutableRefObject<HTMLDivElement | null>).current =
          el;
      }
    },
    [sortableRef],
  );

  if (!meta) {
    return (
      <div className="flex h-full items-center justify-center text-sm text-[var(--color-fg-muted)]">
        Unknown widget: {widgetKey}
      </div>
    );
  }

  const Icon = meta.icon;
  const WidgetComponent = meta.component;

  return (
    <div
      ref={setRefs}
      data-leaf-id={leafId}
      className={cn(
        "relative flex h-full min-h-0 flex-col overflow-hidden rounded-lg",
        "bg-[var(--color-bg)]",
        "transition-shadow",
        isDragging && "opacity-50 ring-2 ring-[var(--color-accent)]",
        dropTarget && !isDragging && "ring-2 ring-[var(--color-accent)]",
      )}
    >
      {/* Title bar */}
      <div className="flex shrink-0 items-center gap-2 rounded-t-lg border border-b-0 border-[var(--color-border)] bg-[var(--color-bg-elevated)] px-2 py-1.5">
        {/* Drag handle */}
        <div
          ref={handleRef}
          className="cursor-grab p-0.5 text-[var(--color-fg-muted)] transition-colors hover:text-[var(--color-fg)] active:cursor-grabbing"
        >
          <GripVertical className="size-3.5" />
        </div>

        <Icon className="size-3.5 text-[var(--color-fg-muted)]" />

        {/* Widget title / swap dropdown */}
        <div className="relative min-w-0 flex-1">
          <button
            type="button"
            onClick={() => setDropdownOpen(!dropdownOpen)}
            className="flex items-center gap-1 truncate text-xs font-medium text-[var(--color-fg)] transition-colors hover:text-[var(--color-accent)]"
          >
            {meta.title}
            {onWidgetChange && (
              <ChevronDown
                className={cn(
                  "size-3 transition-transform",
                  dropdownOpen && "rotate-180",
                )}
              />
            )}
          </button>

          {dropdownOpen && onWidgetChange && (
            <>
              {/* Backdrop to close dropdown */}
              <div
                className="fixed inset-0 z-40"
                onClick={() => setDropdownOpen(false)}
              />
              <div className="absolute top-full left-0 z-50 mt-1 min-w-[160px] overflow-hidden rounded-md border border-[var(--color-border)] bg-[var(--color-bg-elevated)] shadow-md">
                {WIDGET_KEYS.filter((k) => k !== "blank").map((key) => {
                  const wMeta = getWidgetMeta(key);
                  if (!wMeta) return null;
                  const WIcon = wMeta.icon;
                  return (
                    <button
                      key={key}
                      type="button"
                      onClick={() => {
                        onWidgetChange(key);
                        setDropdownOpen(false);
                      }}
                      className={cn(
                        "flex w-full items-center gap-2 px-3 py-1.5 text-xs transition-colors",
                        key === widgetKey
                          ? "bg-[var(--color-bg)] text-[var(--color-accent)]"
                          : "text-[var(--color-fg)] hover:bg-[var(--color-bg)]",
                      )}
                    >
                      <WIcon className="size-3.5" />
                      {wMeta.title}
                    </button>
                  );
                })}
              </div>
            </>
          )}
        </div>

        {/* Close: remove this slot from the layout tree. Only available
            when a remove handler is passed down. */}
        {onWidgetRemove && (
          <button
            type="button"
            onClick={onWidgetRemove}
            title="Remove slot"
            aria-label="Remove slot"
            className="hover:text-destructive shrink-0 rounded p-0.5 text-[var(--color-fg-muted)] transition-colors hover:bg-[var(--color-bg)]"
          >
            <X className="size-3.5" />
          </button>
        )}
      </div>

      {/* Widget content */}
      <div className="min-h-0 flex-1 overflow-auto">
        <WidgetComponent store={store} user={user} />
      </div>

      {/* Split-preview overlay: only shown when this leaf is the drop target.
          Renders two translucent halves plus divider lines at the leaf's
          center, highlighting the half the cursor is over. */}
      {dropTarget && quadrant && <SplitPreview quadrant={quadrant} />}
    </div>
  );
}

interface Quadrant {
  direction: "horizontal" | "vertical";
  /** True if cursor is in the left/top half. */
  first: boolean;
}

function SplitPreview({ quadrant }: { quadrant: Quadrant }) {
  const horizontal = quadrant.direction === "horizontal";
  return (
    <div className="pointer-events-none absolute inset-0 z-20" aria-hidden>
      {/* Highlighted half */}
      <div
        className={cn(
          "absolute bg-[var(--color-accent)]/20",
          horizontal
            ? quadrant.first
              ? "top-0 bottom-0 left-0 w-1/2"
              : "top-0 right-0 bottom-0 w-1/2"
            : quadrant.first
              ? "top-0 right-0 left-0 h-1/2"
              : "right-0 bottom-0 left-0 h-1/2",
        )}
      />
      {/* Divider line at the center */}
      <div
        className={cn(
          "absolute bg-[var(--color-accent)]",
          horizontal
            ? "top-0 bottom-0 left-1/2 w-px -translate-x-1/2"
            : "top-1/2 right-0 left-0 h-px -translate-y-1/2",
        )}
      />
    </div>
  );
}
