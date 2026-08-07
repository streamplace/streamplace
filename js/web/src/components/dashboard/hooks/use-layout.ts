import { useCallback, useState } from "react";
import {
  DEFAULT_LAYOUT,
  type LayoutNode,
  type LayoutPreset,
  validateLayout,
} from "../layout";

const STORAGE_KEY = "dashboard:layout";
const PRESETS_KEY = "dashboard:presets";

function loadFromStorage(): LayoutNode {
  try {
    const stored = localStorage.getItem(STORAGE_KEY);
    if (stored) {
      const parsed = JSON.parse(stored);
      const valid = validateLayout(parsed);
      if (valid) return valid;
    }
  } catch {
    // corrupted storage, fall through to default
  }
  return DEFAULT_LAYOUT;
}

function saveToStorage(layout: LayoutNode): void {
  try {
    localStorage.setItem(STORAGE_KEY, JSON.stringify(layout));
  } catch {
    // storage full or unavailable
  }
}

function loadPresets(): LayoutPreset[] {
  try {
    const stored = localStorage.getItem(PRESETS_KEY);
    if (stored) {
      const parsed = JSON.parse(stored);
      if (Array.isArray(parsed)) {
        return parsed.filter(
          (p): p is LayoutPreset =>
            p &&
            typeof p.name === "string" &&
            validateLayout(p.layout) !== null,
        );
      }
    }
  } catch {}
  return [];
}

function savePresets(presets: LayoutPreset[]): void {
  try {
    localStorage.setItem(PRESETS_KEY, JSON.stringify(presets));
  } catch {}
}

export function useLayout() {
  const [layout, setLayoutState] = useState<LayoutNode>(loadFromStorage);
  const [presets, setPresetsState] = useState<LayoutPreset[]>(loadPresets);

  const setLayout = useCallback((newLayout: LayoutNode) => {
    setLayoutState(newLayout);
    saveToStorage(newLayout);
  }, []);

  const resetLayout = useCallback(() => {
    setLayoutState(DEFAULT_LAYOUT);
    saveToStorage(DEFAULT_LAYOUT);
  }, []);

  const savePreset = useCallback(
    (name: string) => {
      const newPresets = [
        ...presets.filter((p) => p.name !== name),
        { name, layout },
      ];
      setPresetsState(newPresets);
      savePresets(newPresets);
    },
    [layout, presets],
  );

  const loadPreset = useCallback(
    (name: string) => {
      const preset = presets.find((p) => p.name === name);
      if (preset) {
        setLayoutState(preset.layout);
        saveToStorage(preset.layout);
      }
    },
    [presets],
  );

  const deletePreset = useCallback(
    (name: string) => {
      const newPresets = presets.filter((p) => p.name !== name);
      setPresetsState(newPresets);
      savePresets(newPresets);
    },
    [presets],
  );

  return {
    layout,
    setLayout,
    resetLayout,
    presets,
    savePreset,
    loadPreset,
    deletePreset,
  };
}
