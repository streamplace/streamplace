import type { LivestreamStore } from "@streamplace/core";
import { useCallback } from "react";
import type { LayoutNode } from "./layout";
import { ResizeHandle } from "./resize-handle";
import { WidgetLeaf } from "./widget-leaf";

interface LayoutRendererProps {
  node: LayoutNode;
  store: LivestreamStore;
  user?: string;
  /** Called when the layout tree should be updated. */
  onUpdate: (updater: (node: LayoutNode) => LayoutNode) => void;
  /** Path of this node in the tree (array of child indices). */
  path?: number[];
  /** Called when a leaf's widget type is swapped via dropdown. */
  onWidgetChange?: (path: number[], newWidget: string) => void;
  /** Called when a leaf's close button is clicked to remove the slot. */
  onWidgetRemove?: (path: number[]) => void;
  /** Current drop target and pointer position, for split-preview rendering. */
  dragState?: DragState | null;
}

/** Live drag state tracked at the renderer level. */
export interface DragState {
  targetId: string;
  /** Pointer position in viewport coordinates. */
  pointer: { x: number; y: number };
}

/**
 * Adjusts the ratio between two adjacent children in a split node.
 * childIndex is the left/top child; delta is the fraction to shift.
 */
function adjustRatio(
  root: LayoutNode,
  path: number[],
  childIndex: number,
  delta: number,
): LayoutNode {
  if (path.length === 0) {
    if (root.type !== "split") return root;
    const { ratio, children } = root;
    if (childIndex < 0 || childIndex >= ratio.length - 1) return root;

    const minRatio = 0.05;
    const newRatio = [...ratio];
    const shift = Math.max(
      -newRatio[childIndex] + minRatio,
      Math.min(delta, newRatio[childIndex + 1] - minRatio),
    );
    newRatio[childIndex] += shift;
    newRatio[childIndex + 1] -= shift;

    return { ...root, ratio: newRatio };
  }

  if (root.type !== "split") return root;
  const [head, ...rest] = path;
  const newChildren = root.children.map((child, i) =>
    i === head ? adjustRatio(child, rest, childIndex, delta) : child,
  );
  return { ...root, children: newChildren };
}

/**
 * Interleaved renderer: places resize handles between children
 * so they sit on the boundary between two flex children.
 */
export function LayoutRenderer({
  node,
  store,
  user,
  onUpdate,
  path = [],
  onWidgetChange,
  onWidgetRemove,
  dragState = null,
}: LayoutRendererProps) {
  if (node.type === "leaf") {
    const leafId = `leaf-${path.join("-") || "root"}`;
    const leafIndex = path.length === 0 ? 0 : path[path.length - 1];
    const isDropTarget = dragState?.targetId === leafId ? dragState : null;
    return (
      <SortableWidgetLeaf
        id={leafId}
        index={leafIndex}
        widgetKey={node.widget}
        store={store}
        user={user}
        onWidgetChange={
          onWidgetChange
            ? (newWidget) => onWidgetChange(path, newWidget)
            : undefined
        }
        onWidgetRemove={onWidgetRemove ? () => onWidgetRemove(path) : undefined}
        dropTarget={isDropTarget}
      />
    );
  }

  const isHorizontal = node.direction === "horizontal";

  const handleResize = useCallback(
    (childIndex: number, delta: number) => {
      onUpdate((root) => adjustRatio(root, path, childIndex, delta));
    },
    [onUpdate, path],
  );

  const items: React.ReactNode[] = [];

  node.children.forEach((child, i) => {
    const ratio = node.ratio[i] ?? 1 / node.children.length;
    const childPath = [...path, i];

    items.push(
      <div
        key={`child-${i}`}
        className="min-h-0 min-w-0 flex-1"
        style={{ flex: `${ratio} ${ratio} 0%` }}
      >
        <LayoutRenderer
          node={child}
          store={store}
          user={user}
          onUpdate={onUpdate}
          path={childPath}
          onWidgetChange={onWidgetChange}
          onWidgetRemove={onWidgetRemove}
          dragState={dragState}
        />
      </div>,
    );

    if (i < node.children.length - 1) {
      items.push(
        <ResizeHandle
          key={`handle-${i}`}
          direction={node.direction}
          onResize={(delta) => handleResize(i, delta)}
        />,
      );
    }
  });

  return (
    <div
      className={`flex ${isHorizontal ? "flex-row" : "flex-col"} h-full min-h-0 w-full min-w-0`}
    >
      {items}
    </div>
  );
}

// --- Sortable leaf wrapper ---

import { useSortable } from "@dnd-kit/react/sortable";

interface SortableWidgetLeafProps {
  id: string;
  index: number;
  widgetKey: string;
  store: LivestreamStore;
  user?: string;
  onWidgetChange?: (newWidget: string) => void;
  onWidgetRemove?: () => void;
  /** Non-null when this leaf is the current drop target. */
  dropTarget?: DragState | null;
}

function SortableWidgetLeaf({
  id,
  index,
  widgetKey,
  store,
  user,
  onWidgetChange,
  onWidgetRemove,
  dropTarget,
}: SortableWidgetLeafProps) {
  const { ref, isDragging, handleRef } = useSortable({ id, index });

  return (
    <WidgetLeaf
      widgetKey={widgetKey}
      store={store}
      user={user}
      onWidgetChange={onWidgetChange}
      onWidgetRemove={onWidgetRemove}
      isDragging={isDragging}
      handleRef={handleRef}
      sortableRef={ref}
      dropTarget={dropTarget}
      leafId={id}
    />
  );
}

/**
 * Thin wrapper that renders the layout tree. The `DragDropProvider` and
 * drag-event handling are owned by the parent (so the provider can wrap
 * other draggable sources like the widget drawer too). Pass `dragState`
 * down so leaves can render the split-preview overlay.
 */
export function DraggableLayoutRenderer({
  layout,
  store,
  user,
  onUpdate,
  onWidgetChange,
  onWidgetRemove,
  dragState = null,
}: {
  layout: LayoutNode;
  store: LivestreamStore;
  user?: string;
  onUpdate: (updater: (node: LayoutNode) => LayoutNode) => void;
  onWidgetChange?: (path: number[], newWidget: string) => void;
  onWidgetRemove?: (path: number[]) => void;
  dragState?: DragState | null;
}) {
  return (
    <LayoutRenderer
      node={layout}
      store={store}
      user={user}
      onUpdate={onUpdate}
      onWidgetChange={onWidgetChange}
      onWidgetRemove={onWidgetRemove}
      dragState={dragState}
    />
  );
}

/** Convert a leaf ID like "leaf-0-1-0" to a path array [0, 1, 0]. */
export function idToPath(id: string): number[] | null {
  if (!id.startsWith("leaf-")) return null;
  const rest = id.slice(5);
  if (rest === "root") return [];
  const parts = rest.split("-").map(Number);
  if (parts.some(isNaN)) return null;
  return parts;
}

/** Get the widget key at a given path in the tree. */
export function getLeafWidget(root: LayoutNode, path: number[]): string | null {
  if (path.length === 0) {
    return root.type === "leaf" ? root.widget : null;
  }
  if (root.type !== "split") return null;
  const [head, ...rest] = path;
  return getLeafWidget(root.children[head], rest);
}

/**
 * Find a leaf's DOM element by its sortable ID. The sortable stores its
 * element on the underlying draggable/droppable; we walk the document for
 * the matching node. Cheaper alternatives exist (a Map) but this is
 * fine for a small dashboard.
 */
export function findElementById(id: string): HTMLElement | null {
  return document.querySelector<HTMLElement>(`[data-leaf-id="${id}"]`);
}

/**
 * dnd-kit's `Position` is a `ValueHistory`; it tracks `.current` (the
 * latest pointer position) and `.initial` (where the drag started). It
 * does not expose direct `.x`/`.y` properties, so we read `.current`.
 */
export function readPointer(operation: any): { x: number; y: number } | null {
  const pos = operation?.position?.current;
  if (!pos) return null;
  if (typeof pos.x !== "number" || typeof pos.y !== "number") return null;
  return { x: pos.x, y: pos.y };
}
