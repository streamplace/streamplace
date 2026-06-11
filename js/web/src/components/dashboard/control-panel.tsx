import { useFullscreen } from "@/contexts/fullscreen-context";
import { cn } from "@/lib/utils";
import { DragDropProvider, useDraggable } from "@dnd-kit/react";
import type { LivestreamStore } from "@streamplace/core";
import {
  Plus,
  RectangleHorizontal,
  RotateCcw,
  Save,
  Trash2,
  X,
} from "lucide-react";
import { useCallback, useEffect, useRef, useState } from "react";
import { Button } from "../ui/button";
import { useLayout } from "./hooks/use-layout";
import {
  removeLeafAt,
  replaceLeafWithSplit,
  updateLeafAt,
  type LayoutNode,
} from "./layout";
import {
  DraggableLayoutRenderer,
  findElementById,
  getLeafWidget,
  idToPath,
  readPointer,
  type DragState,
} from "./layout-renderer";
import { WIDGET_KEYS, WIDGET_REGISTRY, getWidgetMeta } from "./registry";

interface ControlPanelProps {
  store: LivestreamStore;
  user?: string;
}

/** Prefix for drawer item source ids (e.g. `widget-stream-monitor`). */
const WIDGET_SOURCE_PREFIX = "widget-";

/**
 * Top-level control panel. Owns the drag-and-drop context, the widget
 * drawer, and the layout renderer. The drawer and the layout live inside
 * the same `DragDropProvider` so the user can drag drawer items onto
 * leaves.
 */
export function ControlPanel({ store, user }: ControlPanelProps) {
  const {
    layout,
    setLayout,
    resetLayout,
    presets,
    savePreset,
    loadPreset,
    deletePreset,
  } = useLayout();

  const { theatre, setTheatre } = useFullscreen();

  const [presetName, setPresetName] = useState("");
  const [showSaveInput, setShowSaveInput] = useState(false);
  const [drawerOpen, setDrawerOpen] = useState(false);
  const [dragState, setDragState] = useState<DragState | null>(null);

  const handleUpdate = useCallback(
    (updater: (node: LayoutNode) => LayoutNode) => {
      setLayout(updater(layout));
    },
    [layout, setLayout],
  );

  const handleWidgetChange = useCallback(
    (path: number[], newWidget: string) => {
      if (!(newWidget in WIDGET_REGISTRY)) return;
      setLayout(updateLeafAt(layout, path, newWidget));
    },
    [layout, setLayout],
  );

  const handleWidgetRemove = useCallback(
    (path: number[]) => {
      setLayout(removeLeafAt(layout, path));
    },
    [layout, setLayout],
  );

  const handleSavePreset = useCallback(() => {
    const name = presetName.trim();
    if (!name) return;
    savePreset(name);
    setPresetName("");
    setShowSaveInput(false);
  }, [presetName, savePreset]);

  // Live drag tracking. `onDragOver` fires on target change; `onDragMove`
  // fires on every pointer move (so the overlay tracks the cursor even
  // when it stays within a single leaf).
  const handleDragOver = useCallback((event: any) => {
    const target = event.operation?.target;
    const pointer = readPointer(event.operation);
    if (target && pointer) {
      setDragState({ targetId: String(target.id), pointer });
    } else {
      setDragState(null);
    }
  }, []);

  const handleDragMove = useCallback((event: any) => {
    const target = event.operation?.target;
    const pointer = readPointer(event.operation);
    if (target && pointer) {
      setDragState({ targetId: String(target.id), pointer });
    }
  }, []);

  /**
   * Unified drop handler. Supports two kinds of source:
   * - leaf source (`leaf-{path}`): move the widget — split the target
   *   and remove the source from its original slot
   * - drawer source (`widget-{key}`): add the widget — split the target
   *   with the dragged widget, no source removal
   */
  const handleDragEnd = useCallback(
    (event: any) => {
      setDragState(null);
      if (event.canceled) return;
      const { source, target } = event.operation;
      if (!source || !target) return;

      const sourceId = String(source.id);
      const targetId = String(target.id);
      if (sourceId === targetId) return;

      const isWidgetSource = sourceId.startsWith(WIDGET_SOURCE_PREFIX);

      // Resolve the source widget and (for leaf sources) its path.
      let sourceWidget: string;
      let sourcePath: number[] | null = null;
      if (isWidgetSource) {
        sourceWidget = sourceId.slice(WIDGET_SOURCE_PREFIX.length);
        if (!(sourceWidget in WIDGET_REGISTRY)) return;
      } else {
        sourcePath = idToPath(sourceId);
        if (!sourcePath) return;
        const widget = getLeafWidget(layout, sourcePath);
        if (!widget) return;
        sourceWidget = widget;
      }

      const targetPath = idToPath(targetId);
      if (!targetPath) return;
      const targetWidget = getLeafWidget(layout, targetPath);

      const targetEl = findElementById(targetId);
      const pointer = readPointer(event.operation);
      if (!targetEl || !pointer) return;

      const rect = targetEl.getBoundingClientRect();
      if (rect.width === 0 || rect.height === 0) return;

      const relX = (pointer.x - rect.left) / rect.width;
      const relY = (pointer.y - rect.top) / rect.height;
      if (
        !Number.isFinite(relX) ||
        !Number.isFinite(relY) ||
        relX < 0 ||
        relX > 1 ||
        relY < 0 ||
        relY > 1
      ) {
        return;
      }

      const finalSourceWidget = sourceWidget;
      let nextLayout: LayoutNode;

      // Dropping onto a blank slot just fills it — no split, since a
      // split with a blank in one half would be weird. For leaf
      // sources we also remove the source so the widget effectively
      // moves into the empty slot.
      if (targetWidget === "blank") {
        const filled = updateLeafAt(layout, targetPath, finalSourceWidget);
        nextLayout = sourcePath ? removeLeafAt(filled, sourcePath) : filled;
      } else {
        // Dominant axis determines split direction. The sign of the
        // displacement chooses which side the source widget lands on.
        const horizontalDominant = Math.abs(relX - 0.5) >= Math.abs(relY - 0.5);
        const direction = horizontalDominant ? "horizontal" : "vertical";
        const sourceFirst = horizontalDominant ? relX < 0.5 : relY < 0.5;

        const withSplit = replaceLeafWithSplit(
          layout,
          targetPath,
          finalSourceWidget,
          direction,
          sourceFirst,
        );
        nextLayout = sourcePath
          ? removeLeafAt(withSplit, sourcePath)
          : withSplit;
      }

      setLayout(nextLayout);
    },
    [layout, setLayout],
  );

  return (
    <DragDropProvider
      onDragOver={handleDragOver}
      onDragMove={handleDragMove}
      onDragEnd={handleDragEnd}
    >
      <div className="flex flex-col h-full min-h-0 relative">
        {/* Toolbar */}
        {!theatre && (
          <div className="flex items-center gap-2 px-3 py-2 border-b border-[var(--color-border)] bg-[var(--color-bg-elevated)] shrink-0">
            {/* Preset selector */}
            {presets.length > 0 && (
              <select
                value=""
                onChange={(e) => {
                  if (e.target.value) loadPreset(e.target.value);
                }}
                className="h-7 px-2 text-xs rounded border border-[var(--color-border)] bg-[var(--color-bg)] text-[var(--color-fg)]"
              >
                <option value="">Load preset…</option>
                {presets.map((p) => (
                  <option key={p.name} value={p.name}>
                    {p.name}
                  </option>
                ))}
              </select>
            )}

            {/* Save preset */}
            {showSaveInput ? (
              <div className="flex items-center gap-1">
                <input
                  value={presetName}
                  onChange={(e) => setPresetName(e.target.value)}
                  onKeyDown={(e) => {
                    if (e.key === "Enter") handleSavePreset();
                    if (e.key === "Escape") setShowSaveInput(false);
                  }}
                  placeholder="Preset name…"
                  autoFocus
                  className="h-7 px-2 text-xs rounded border border-[var(--color-border)] bg-[var(--color-bg)] text-[var(--color-fg)] w-32"
                />
                <button
                  type="button"
                  onClick={handleSavePreset}
                  className="h-7 px-2 text-xs rounded bg-[var(--color-accent)] text-[var(--color-accent-fg)] hover:bg-[var(--color-accent-hover)] transition-colors"
                >
                  Save
                </button>
              </div>
            ) : (
              <button
                type="button"
                onClick={() => setShowSaveInput(true)}
                className="h-7 px-2 text-xs rounded border border-[var(--color-border)] hover:border-[var(--color-border-strong)] text-[var(--color-fg-muted)] hover:text-[var(--color-fg)] transition-colors flex items-center gap-1"
              >
                <Save className="size-3" />
                Save Preset
              </button>
            )}

            {/* Preset management */}
            {presets.length > 0 && (
              <div className="flex items-center gap-1">
                {presets.map((p) => (
                  <div
                    key={p.name}
                    className="flex items-center gap-0.5 h-7 px-1.5 text-xs rounded bg-[var(--color-bg)] border border-[var(--color-border)]"
                  >
                    <button
                      type="button"
                      onClick={() => loadPreset(p.name)}
                      className="text-[var(--color-fg-muted)] hover:text-[var(--color-fg)] transition-colors"
                    >
                      {p.name}
                    </button>
                    <button
                      type="button"
                      onClick={() => deletePreset(p.name)}
                      className="text-[var(--color-fg-muted)] hover:text-red-400 transition-colors"
                    >
                      <Trash2 className="size-3" />
                    </button>
                  </div>
                ))}
              </div>
            )}

            <div className="flex-1" />

            {/* Add Widget toggle */}
            <button
              type="button"
              id="drawer-toggle-button"
              onClick={() => setDrawerOpen((v) => !v)}
              className={cn(
                "h-7 px-2 text-xs rounded border transition-colors flex items-center gap-1",
                drawerOpen
                  ? "bg-[var(--color-accent)]/15 border-[var(--color-accent)] text-[var(--color-accent)]"
                  : "border-[var(--color-border)] hover:border-[var(--color-border-strong)] text-[var(--color-fg-muted)] hover:text-[var(--color-fg)]",
              )}
              aria-pressed={drawerOpen}
            >
              {drawerOpen ? (
                <X className="size-3" />
              ) : (
                <Plus className="size-3" />
              )}
              {drawerOpen ? "Close" : "Add Widget"}
            </button>

            {/* Reset */}
            <button
              type="button"
              onClick={resetLayout}
              className="h-7 px-2 text-xs rounded border border-[var(--color-border)] hover:border-[var(--color-border-strong)] text-[var(--color-fg-muted)] hover:text-[var(--color-fg)] transition-colors flex items-center gap-1"
              title="Reset to default layout"
            >
              <RotateCcw className="size-3" />
              Reset
            </button>

            {/* Theatre mode */}
            <button
              type="button"
              onClick={() => setTheatre(true)}
              className="h-7 px-2 text-xs rounded border border-[var(--color-border)] hover:border-[var(--color-border-strong)] text-[var(--color-fg-muted)] hover:text-[var(--color-fg)] transition-colors flex items-center gap-1"
              title="Theatre mode"
              aria-label="Theatre mode"
            >
              <RectangleHorizontal className="size-3" />
              Theatre
            </button>
          </div>
        )}

        {/* Drawer (slides down between toolbar and layout) */}
        {!theatre && (
          <WidgetDrawer
            open={drawerOpen}
            onClose={() => setDrawerOpen(false)}
          />
        )}

        {/* Layout area */}
        <div className="flex-1 min-h-0 p-1.5">
          <DraggableLayoutRenderer
            layout={layout}
            store={store}
            user={user}
            onUpdate={handleUpdate}
            onWidgetChange={handleWidgetChange}
            onWidgetRemove={handleWidgetRemove}
            dragState={dragState}
          />
        </div>

        {/* Theatre-mode exit button: floating top-right inside the control
            panel so it's reachable after the toolbar and drawer are gone. */}
        {theatre && (
          <button
            type="button"
            onClick={() => setTheatre(false)}
            aria-label="Exit theatre mode"
            title="Exit theatre mode"
            className="absolute top-2 right-10 z-20 h-7 px-2 text-xs rounded border border-[var(--color-border)] bg-[var(--color-bg-elevated)]/80 backdrop-blur text-[var(--color-fg-muted)] hover:text-[var(--color-fg)] hover:border-[var(--color-border-strong)] transition-colors flex items-center gap-1 opacity-50 hover:opacity-100"
          >
            <X className="size-3" />
            Exit Theatre
          </button>
        )}
      </div>
    </DragDropProvider>
  );
}

/**
 * Drawer with draggable cards for every widget in the registry (except
 * `blank`, which is only a placeholder, not a real widget). Drag a card
 * onto a leaf in the layout to add it.
 * Positioned as a 'drawer' that slides down from the toolbar.
 */
function WidgetDrawer({
  open,
  onClose,
}: {
  open: boolean;
  onClose: () => void;
}) {
  const [exiting, setExiting] = useState(false);
  const exitingRef = useRef(false);

  useEffect(() => {
    if (exiting) {
      const timer = setTimeout(() => {
        setExiting(false);
        exitingRef.current = false;
        onClose();
      }, 500);
      return () => clearTimeout(timer);
    }
  }, [exiting, onClose]);

  const handleClose = useCallback(() => {
    if (!exitingRef.current) {
      exitingRef.current = true;
      setExiting(true);
    }
  }, []);

  const outerDivRef = useCallback(
    (el: HTMLDivElement | null) => {
      if (el) {
        const handleClick = (e: MouseEvent) => {
          if (!el.contains(e.target as Node)) {
            if (
              e.target instanceof HTMLElement &&
              e.target.closest("#drawer-toggle-button")
            ) {
              return;
            }
            handleClose();
          }
        };
        document.addEventListener("mousedown", handleClick);
        return () => {
          document.removeEventListener("mousedown", handleClick);
        };
      }
    },
    [handleClose],
  );

  if (!open && !exiting) return null;

  return (
    <>
      <div
        className={cn(
          "absolute top-0 left-0 right-0 mx-auto max-w-2xl w-full z-10",
          exiting ? "animate-fade-out-up" : "animate-fade-in-down",
        )}
        ref={outerDivRef}
      >
        <div className="flex flex-col gap-1 px-3 py-2 border-b border-[var(--color-border)] bg-[var(--color-bg)] rounded-b-lg shrink-0">
          <div className="flex items-center justify-between">
            <div className="text-[var(--color-fg-muted)]">
              Drag a widget to add it
            </div>
            <Button variant="ghost" size="icon" onClick={handleClose}>
              <X className="size-4 text-[var(--color-fg-muted)]" />
            </Button>
          </div>
          <div className="flex flex-wrap gap-1.5">
            {WIDGET_KEYS.filter((k) => k !== "blank").map((key) => (
              <DrawerWidgetCard key={key} widgetKey={key} />
            ))}
          </div>
        </div>
      </div>
    </>
  );
}

function DrawerWidgetCard({ widgetKey }: { widgetKey: string }) {
  const { ref, isDragging } = useDraggable({
    id: `${WIDGET_SOURCE_PREFIX}${widgetKey}`,
  });
  const meta = getWidgetMeta(widgetKey);
  if (!meta) return null;
  const Icon = meta.icon;
  return (
    <div
      ref={ref}
      className={cn(
        "flex items-center gap-1.5 px-2.5 py-1.5 rounded-md border border-[var(--color-border)] bg-[var(--color-bg-elevated)] cursor-grab active:cursor-grabbing select-none transition-opacity hover:border-[var(--color-border-strong)]",
        isDragging && "opacity-50",
      )}
      style={
        isDragging ? { color: meta.color, borderColor: meta.color } : undefined
      }
    >
      <Icon className="size-3.5 text-[var(--color-fg-muted)]" />
      <span className="text-xs font-medium">{meta.title}</span>
    </div>
  );
}
