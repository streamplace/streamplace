import {
  CaptionDisplayPrefs,
  DEFAULT_CAPTION_PREFS,
  parseCaptionPrefs,
} from "@streamplace/core";
import { StateCreator } from "zustand";
import { storage } from "../../storage";
import { AppStore } from "../index";

// Same keys as the app's streamplace store, so a viewer's caption
// settings carry over between the two web frontends.
const CAPTIONS_ENABLED_KEY = "captionsEnabled";
const CAPTION_LANGUAGE_KEY = "captionLanguage";
const CAPTION_PREFS_KEY = "captionPrefs";

export interface CaptionsSlice {
  captionsEnabled: boolean;
  /** Language of the last caption track the viewer picked. */
  captionLanguage: string | null;
  captionPrefs: CaptionDisplayPrefs;
  setCaptionsEnabled: (enabled: boolean) => void;
  setCaptionLanguage: (language: string | null) => void;
  setCaptionPrefs: (prefs: CaptionDisplayPrefs) => void;
}

export const createCaptionsSlice: StateCreator<
  AppStore,
  [],
  [],
  CaptionsSlice
> = (set) => ({
  captionsEnabled: false,
  captionLanguage: null,
  captionPrefs: DEFAULT_CAPTION_PREFS,

  setCaptionsEnabled: (enabled) => {
    set({ captionsEnabled: enabled });
    storage
      .setItem(CAPTIONS_ENABLED_KEY, enabled.toString())
      .catch(console.error);
  },
  setCaptionLanguage: (language) => {
    set({ captionLanguage: language });
    const write = language
      ? storage.setItem(CAPTION_LANGUAGE_KEY, language)
      : storage.removeItem(CAPTION_LANGUAGE_KEY);
    write.catch(console.error);
  },
  setCaptionPrefs: (prefs) => {
    set({ captionPrefs: prefs });
    storage
      .setItem(CAPTION_PREFS_KEY, JSON.stringify(prefs))
      .catch(console.error);
  },
});

/** Loads persisted caption settings into the store; see hydrateDanmuSettings. */
export function hydrateCaptionSettings(store: {
  setState: (partial: Partial<CaptionsSlice>) => void;
}) {
  void (async () => {
    try {
      const [enabled, language, prefs] = await Promise.all([
        storage.getItem(CAPTIONS_ENABLED_KEY),
        storage.getItem(CAPTION_LANGUAGE_KEY),
        storage.getItem(CAPTION_PREFS_KEY),
      ]);
      store.setState({
        captionsEnabled: enabled === "true",
        captionLanguage: language || null,
        captionPrefs: parseCaptionPrefs(prefs),
      });
    } catch (error) {
      console.error("Failed to load caption settings from storage:", error);
    }
  })();
}
