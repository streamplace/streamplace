export interface WidgetNode {
  type: "leaf";
  widget: string;
  config?: Record<string, unknown>;
}

export interface SplitNode {
  type: "split";
  direction: "horizontal" | "vertical";
  ratio: number[];
  children: LayoutNode[];
}

export type LayoutNode = WidgetNode | SplitNode;
export type LayoutPreset = { name: string; layout: LayoutNode };

export const DEFAULT_LAYOUT: LayoutNode = {
  type: "split",
  direction: "horizontal",
  ratio: [0.5, 0.25, 0.25],
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
      ratio: [0.15, 0.85],
      children: [
        { type: "leaf", widget: "multistream" },
        { type: "leaf", widget: "chat" },
      ],
    },
    {
      type: "split",
      direction: "vertical",
      ratio: [0.6, 0.4],
      children: [
        { type: "leaf", widget: "stream-info" },
        { type: "leaf", widget: "moderators" },
      ],
    },
  ],
};

function normalizeRatio(value: unknown, length: number): number[] | null {
  if (!Array.isArray(value) || value.length !== length) return null;
  if (
    value.some(
      (number) =>
        typeof number !== "number" || !Number.isFinite(number) || number < 0,
    )
  )
    return null;
  let largest = 0;
  for (const number of value) largest = Math.max(largest, number);
  if (largest === 0) return value.map(() => 1 / length);
  const scaled = value.map((number) => number / largest);
  const total = scaled.reduce((sum, number) => sum + number, 0);
  return scaled.map((number) => number / total);
}

export function validateLayout(value: unknown, depth = 0): LayoutNode | null {
  if (!value || typeof value !== "object" || Array.isArray(value) || depth > 64)
    return null;
  const node = value as Record<string, unknown>;
  if (node.type === "leaf") {
    if (typeof node.widget !== "string" || !node.widget) return null;
    if (
      node.config != null &&
      (typeof node.config !== "object" || Array.isArray(node.config))
    )
      return null;
    return {
      type: "leaf",
      widget: node.widget,
      ...(node.config != null
        ? { config: node.config as Record<string, unknown> }
        : {}),
    };
  }
  if (
    node.type !== "split" ||
    (node.direction !== "horizontal" && node.direction !== "vertical") ||
    !Array.isArray(node.children) ||
    node.children.length < 2
  )
    return null;
  const ratio = normalizeRatio(node.ratio, node.children.length);
  if (!ratio) return null;
  const children = node.children.map((child) =>
    validateLayout(child, depth + 1),
  );
  if (children.some((child) => child === null)) return null;
  return {
    type: "split",
    direction: node.direction,
    ratio,
    children: children as LayoutNode[],
  };
}

function widgetAt(
  root: LayoutNode,
  path: readonly number[],
): WidgetNode | null {
  let node = root;
  for (const index of path) {
    if (
      node.type !== "split" ||
      !Number.isInteger(index) ||
      index < 0 ||
      index >= node.children.length
    )
      return null;
    node = node.children[index];
  }
  return node.type === "leaf" ? node : null;
}

function updateAt(
  root: LayoutNode,
  path: readonly number[],
  update: (node: LayoutNode) => LayoutNode,
): LayoutNode {
  if (path.length === 0) return update(root);
  if (root.type !== "split") return root;
  const [index, ...rest] = path;
  if (!Number.isInteger(index) || index < 0 || index >= root.children.length)
    return root;
  const child = updateAt(root.children[index], rest, update);
  if (child === root.children[index]) return root;
  const children = root.children.slice();
  children[index] = child;
  return { ...root, children };
}

export function insertWidget(
  root: LayoutNode,
  path: readonly number[],
  widget: WidgetNode,
  direction: SplitNode["direction"],
  first: boolean,
): LayoutNode {
  return updateAt(root, path, (node) => {
    if (node.type !== "leaf") return node;
    if (node.widget === "blank") return widget;
    return {
      type: "split",
      direction,
      ratio: [0.5, 0.5],
      children: first ? [widget, node] : [node, widget],
    };
  });
}

export function removeWidget(
  root: LayoutNode,
  path: readonly number[],
): LayoutNode {
  if (!path.length)
    return root.type === "leaf" && root.widget !== "blank"
      ? { type: "leaf", widget: "blank" }
      : root;
  if (!widgetAt(root, path)) return root;
  const parent = path.slice(0, -1);
  const index = path[path.length - 1];
  return updateAt(root, parent, (node) => {
    if (node.type !== "split") return node;
    const children = node.children.filter((_, i) => i !== index);
    if (children.length === 1) return children[0];
    const ratio = normalizeRatio(
      node.ratio.filter((_, i) => i !== index),
      children.length,
    )!;
    return { ...node, children, ratio };
  });
}

export function moveWidget(
  root: LayoutNode,
  source: readonly number[],
  target: readonly number[],
  direction: SplitNode["direction"],
  first: boolean,
): LayoutNode {
  const widget = widgetAt(root, source);
  if (
    !widget ||
    !widgetAt(root, target) ||
    (source.length === target.length &&
      source.every((value, i) => value === target[i]))
  )
    return root;
  return removeWidget(
    insertWidget(root, target, widget, direction, first),
    source,
  );
}

export function updateRatio(
  root: LayoutNode,
  path: readonly number[],
  ratio: number[],
): LayoutNode {
  return updateAt(root, path, (node) => {
    if (node.type !== "split") return node;
    const normalized = normalizeRatio(ratio, node.children.length);
    return normalized ? { ...node, ratio: normalized } : node;
  });
}
