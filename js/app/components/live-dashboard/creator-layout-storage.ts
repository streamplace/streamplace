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

export function loadPresets(): LayoutPreset[] {
  try {
    const stored = localStorage.getItem(PRESETS_KEY);
    const value = stored ? JSON.parse(stored) : [];
    if (!Array.isArray(value)) return [];
    return value.flatMap((item) => {
      if (!item || typeof item !== "object" || typeof item.name !== "string")
        return [];
      const layout = validateLayout(item.layout);
      return layout ? [{ name: item.name, layout }] : [];
    });
  } catch {
    return [];
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
