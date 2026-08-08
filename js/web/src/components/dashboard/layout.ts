/**
 * Layout data model for the dashboard control panel.
 *
 * The layout is a recursive tree where internal nodes are "splits"
 * (horizontal or vertical containers with fractional ratios) and
 * leaf nodes are "widgets" (identified by a string key into the
 * widget registry).
 *
 * The entire tree serializes to/from JSON for localStorage persistence.
 */

// --- Types ---

export interface SplitNode {
  type: "split";
  direction: "horizontal" | "vertical";
  /** Fractional sizes for each child. Must sum to ~1.0. */
  ratio: number[];
  children: LayoutNode[];
}

export interface LeafNode {
  type: "leaf";
  /** Key into the widget registry. */
  widget: string;
  /** Optional per-widget configuration. */
  config?: Record<string, unknown>;
}

export type LayoutNode = SplitNode | LeafNode;

export interface LayoutPreset {
  name: string;
  layout: LayoutNode;
}

// --- Default layout ---

export const DEFAULT_LAYOUT: LayoutNode = {
  type: "split",
  direction: "horizontal",
  ratio: [0.65, 0.35],
  children: [
    {
      type: "split",
      direction: "vertical",
      ratio: [0.6, 0.4],
      children: [
        { type: "leaf", widget: "stream-monitor" },
        { type: "leaf", widget: "stream-health" },
      ],
    },
    {
      type: "split",
      direction: "vertical",
      ratio: [0.5, 0.3, 0.2],
      children: [
        { type: "leaf", widget: "chat" },
        { type: "leaf", widget: "stream-info" },
        { type: "leaf", widget: "multistream" },
      ],
    },
  ],
};

// --- Validation ---

function isValidDirection(d: unknown): d is "horizontal" | "vertical" {
  return d === "horizontal" || d === "vertical";
}

/**
 * Validates and normalizes a parsed JSON value into a LayoutNode.
 * Returns null if the value is not a valid layout.
 */
export function validateLayout(value: unknown): LayoutNode | null {
  if (!value || typeof value !== "object") return null;
  const obj = value as Record<string, unknown>;

  if (obj.type === "leaf") {
    if (typeof obj.widget !== "string") return null;
    return {
      type: "leaf",
      widget: obj.widget,
      config:
        obj.config && typeof obj.config === "object"
          ? (obj.config as Record<string, unknown>)
          : undefined,
    };
  }

  if (obj.type === "split") {
    if (!isValidDirection(obj.direction)) return null;
    if (!Array.isArray(obj.ratio) || !Array.isArray(obj.children)) return null;
    if (obj.ratio.length < 2 || obj.ratio.length !== obj.children.length)
      return null;

    const children = obj.children
      .map(validateLayout)
      .filter((c): c is LayoutNode => c !== null);

    if (children.length < 2) return null;

    // Normalize ratios to sum to 1
    const total = obj.ratio.reduce(
      (sum: number, r: unknown) => sum + (typeof r === "number" ? r : 0),
      0,
    );
    const ratio =
      total > 0
        ? (obj.ratio as number[]).map((r) => r / total)
        : children.map(() => 1 / children.length);

    return { type: "split", direction: obj.direction, ratio, children };
  }

  return null;
}

// --- Tree utilities ---

/** Collect all widget keys used in the layout. */
export function collectWidgets(node: LayoutNode): string[] {
  if (node.type === "leaf") return [node.widget];
  return node.children.flatMap(collectWidgets);
}

/**
 * Update a leaf node at a given path (array of child indices).
 * Returns a new tree with the leaf's widget key replaced.
 */
export function updateLeafAt(
  root: LayoutNode,
  path: number[],
  widget: string,
): LayoutNode {
  if (path.length === 0) {
    if (root.type === "leaf") return { ...root, widget };
    return root;
  }
  if (root.type !== "split") return root;

  const [head, ...rest] = path;
  const newChildren = root.children.map((child, i) =>
    i === head ? updateLeafAt(child, rest, widget) : child,
  );
  return { ...root, children: newChildren };
}

/**
 * Replace a leaf at the given path with a split node containing the leaf's
 * original widget and a new source widget. Used for the "drop to split"
 * interaction in the control panel: dropping a widget onto a leaf turns
 * that leaf into a 50/50 split.
 */
export function replaceLeafWithSplit(
  root: LayoutNode,
  path: number[],
  sourceWidget: string,
  direction: "horizontal" | "vertical",
  sourceFirst: boolean,
): LayoutNode {
  if (path.length === 0) {
    if (root.type !== "leaf") return root;
    const original = root.widget;
    return {
      type: "split",
      direction,
      ratio: [0.5, 0.5],
      children: sourceFirst
        ? [
            { type: "leaf", widget: sourceWidget },
            { type: "leaf", widget: original },
          ]
        : [
            { type: "leaf", widget: original },
            { type: "leaf", widget: sourceWidget },
          ],
    };
  }
  if (root.type !== "split") return root;
  const [head, ...rest] = path;
  return {
    ...root,
    children: root.children.map((child, i) =>
      i === head
        ? replaceLeafWithSplit(
            child,
            rest,
            sourceWidget,
            direction,
            sourceFirst,
          )
        : child,
    ),
  };
}

/**
 * Remove a leaf at the given path. Any split that ends up with fewer than
 * two children is collapsed into its surviving child, so the tree always
 * remains a valid layout (splits have >= 2 children).
 *
 * If the path points at the root leaf (path length 0), the root is
 * returned unchanged; there's no sensible empty layout to produce.
 */
export function removeLeafAt(root: LayoutNode, path: number[]): LayoutNode {
  if (path.length === 0) return root;
  if (root.type !== "split") return root;

  const [head, ...rest] = path;
  if (head < 0 || head >= root.children.length) return root;

  if (rest.length === 0) {
    // Direct child of this split is the leaf to remove.
    const newChildren = root.children.filter((_, i) => i !== head);
    const newRatio = root.ratio.filter((_, i) => i !== head);
    const total = newRatio.reduce((sum, r) => sum + r, 0);
    const ratio =
      total > 0
        ? newRatio.map((r) => r / total)
        : newChildren.map(() => 1 / Math.max(newChildren.length, 1));

    if (newChildren.length === 1) {
      return newChildren[0];
    }
    if (newChildren.length === 0) {
      // Edge case: split had one child and we removed it. Return a
      // placeholder blank so the caller still gets a valid tree.
      return { type: "leaf", widget: "blank" };
    }
    return { ...root, children: newChildren, ratio };
  }

  // Recurse into the child at `head`. The recursion may collapse that
  // child to a single node, but as long as we still have 2+ children
  // here, the parent split stays valid.
  const newChildren = root.children.map((child, i) =>
    i === head ? removeLeafAt(child, rest) : child,
  );
  return { ...root, children: newChildren };
}

/**
 * Update the ratio of a split node at a given path.
 */
export function updateRatioAt(
  root: LayoutNode,
  path: number[],
  newRatio: number[],
): LayoutNode {
  if (path.length === 0) {
    if (root.type === "split") return { ...root, ratio: newRatio };
    return root;
  }
  if (root.type !== "split") return root;

  const [head, ...rest] = path;
  const newChildren = root.children.map((child, i) =>
    i === head ? updateRatioAt(child, rest, newRatio) : child,
  );
  return { ...root, children: newChildren };
}

/** Assign unique IDs to each leaf for dnd-kit. Returns a flat list. */
export interface LeafDescriptor {
  id: string;
  path: number[];
  widget: string;
}

export function enumerateLeaves(
  node: LayoutNode,
  path: number[] = [],
): LeafDescriptor[] {
  if (node.type === "leaf") {
    return [
      {
        id: `leaf-${path.join("-") || "root"}`,
        path,
        widget: node.widget,
      },
    ];
  }
  return node.children.flatMap((child, i) =>
    enumerateLeaves(child, [...path, i]),
  );
}
