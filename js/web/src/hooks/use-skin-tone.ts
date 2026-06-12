// Skin tone preference for emoji insertion.
//
// We store frimousse's `SkinTone` string directly so the same value can be
// fed to the visual picker and read back from the shortcode autocomplete
// without translation. The integer index 0..5 is only used at the boundary
// with the emojibase data's `s[]` skin array (see `skinToneIndex`).

import type { SkinTone } from "frimousse";
import { useCallback, useState } from "react";

const STORAGE_KEY = "streamplace:skinTone";

// Reference emojis used to render the tray and trigger. 👋 is the standard
// reference for skin tone pickers; its skin variants cover Fitzpatrick 1..5.
export const SKIN_TONES: ReadonlyArray<{
  key: SkinTone;
  label: string;
  emoji: string;
}> = [
  { key: "none", label: "Default", emoji: "👋" },
  { key: "light", label: "Light", emoji: "👋🏻" },
  { key: "medium-light", label: "Medium-light", emoji: "👋🏼" },
  { key: "medium", label: "Medium", emoji: "👋🏽" },
  { key: "medium-dark", label: "Medium-dark", emoji: "👋🏾" },
  { key: "dark", label: "Dark", emoji: "👋🏿" },
];

const KEY_TO_INDEX: Record<SkinTone, number> = {
  none: 0,
  light: 1,
  "medium-light": 2,
  medium: 3,
  "medium-dark": 4,
  dark: 5,
};

const VALID_KEYS = new Set<SkinTone>(SKIN_TONES.map((t) => t.key));

function readStored(): SkinTone {
  if (typeof localStorage === "undefined") return "none";
  const raw = localStorage.getItem(STORAGE_KEY);
  if (raw && (VALID_KEYS as Set<string>).has(raw)) return raw as SkinTone;
  return "none";
}

function writeStored(value: SkinTone) {
  if (typeof localStorage === "undefined") return;
  localStorage.setItem(STORAGE_KEY, value);
}

/** Convert a frimousse SkinTone to the array index used by emojibase's `s[]`. */
export function skinToneIndex(tone: SkinTone): number {
  return KEY_TO_INDEX[tone] ?? 0;
}

export function useSkinTone(): [SkinTone, (next: SkinTone) => void] {
  // Lazy init: read from localStorage on the first render so the value
  // is correct from the start. No useEffect needed.
  const [tone, setToneState] = useState<SkinTone>(readStored);

  const setTone = useCallback((next: SkinTone) => {
    setToneState(next);
    writeStored(next);
  }, []);

  return [tone, setTone];
}

/** Returns the reference emoji for a given skin tone. */
export function skinToneEmoji(tone: SkinTone): string {
  return SKIN_TONES.find((t) => t.key === tone)?.emoji ?? SKIN_TONES[0].emoji;
}
