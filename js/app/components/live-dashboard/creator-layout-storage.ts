import {
  DEFAULT_LAYOUT,
  validateLayout,
  type LayoutNode,
  type LayoutPreset,
} from "./creator-layout";

const LAYOUT_KEY = "dashboard:layout";
const PRESETS_KEY = "dashboard:presets";

export function loadLayout(): LayoutNode {
  try {
    const stored = localStorage.getItem(LAYOUT_KEY);
    return stored
      ? (validateLayout(JSON.parse(stored)) ?? DEFAULT_LAYOUT)
      : DEFAULT_LAYOUT;
  } catch {
    return DEFAULT_LAYOUT;
  }
}

export function saveLayout(layout: LayoutNode): boolean {
  try {
    localStorage.setItem(LAYOUT_KEY, JSON.stringify(layout));
    return true;
  } catch {
    return false;
  }
}

export function loadPresets(): {
  presets: LayoutPreset[];
  unreadable: boolean;
} {
  try {
    const stored = localStorage.getItem(PRESETS_KEY);
    const value = stored === null ? [] : JSON.parse(stored);
    if (!Array.isArray(value)) return { presets: [], unreadable: true };
    const presets: LayoutPreset[] = [];
    let unreadable = false;
    for (const item of value) {
      const layout =
        item && typeof item === "object" && typeof item.name === "string"
          ? validateLayout(item.layout)
          : null;
      if (layout) presets.push({ name: item.name, layout });
      else unreadable = true;
    }
    return { presets, unreadable };
  } catch {
    return { presets: [], unreadable: true };
  }
}

export function savePresets(presets: LayoutPreset[]): boolean {
  try {
    localStorage.setItem(PRESETS_KEY, JSON.stringify(presets));
    return true;
  } catch {
    return false;
  }
}
