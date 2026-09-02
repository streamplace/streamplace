// Danmu settings, ported from js/components/src/streamplace-store
// (useDanmuSettings). Persisted to localStorage with the same keys the
// app uses so a user's danmu preferences carry across frontends.
import { StateCreator } from "zustand";
import { storage } from "../../storage";
import { AppStore } from "../index";

const DANMU_UNLOCKED_KEY = "danmuUnlocked";
const DANMU_ENABLED_KEY = "danmuEnabled";
const DANMU_OPACITY_KEY = "danmuOpacity";
const DANMU_SPEED_KEY = "danmuSpeed";
const DANMU_LANE_COUNT_KEY = "danmuLaneCount";
const DANMU_MAX_MESSAGES_KEY = "danmuMaxMessages";

export interface DanmuSlice {
  danmuUnlocked: boolean;
  danmuEnabled: boolean;
  danmuOpacity: number;
  danmuSpeed: number;
  danmuLaneCount: number;
  danmuMaxMessages: number;
  setDanmuUnlocked: (unlocked: boolean) => void;
  setDanmuEnabled: (enabled: boolean) => void;
  setDanmuOpacity: (opacity: number) => void;
  setDanmuSpeed: (speed: number) => void;
  setDanmuLaneCount: (laneCount: number) => void;
  setDanmuMaxMessages: (maxMessages: number) => void;
}

export const createDanmuSlice: StateCreator<AppStore, [], [], DanmuSlice> = (
  set,
) => ({
  danmuUnlocked: false,
  danmuEnabled: false,
  danmuOpacity: 80,
  danmuSpeed: 1,
  danmuLaneCount: 12,
  danmuMaxMessages: 50,

  setDanmuUnlocked: (unlocked) => {
    set({ danmuUnlocked: unlocked });
    storage
      .setItem(DANMU_UNLOCKED_KEY, unlocked.toString())
      .catch(console.error);
  },
  setDanmuEnabled: (enabled) => {
    set({ danmuEnabled: enabled });
    storage.setItem(DANMU_ENABLED_KEY, enabled.toString()).catch(console.error);
  },
  setDanmuOpacity: (opacity) => {
    const clamped = Math.max(0, Math.min(100, opacity));
    set({ danmuOpacity: clamped });
    storage.setItem(DANMU_OPACITY_KEY, clamped.toString()).catch(console.error);
  },
  setDanmuSpeed: (speed) => {
    const clamped = Math.max(0.1, Math.min(3, speed));
    set({ danmuSpeed: clamped });
    storage.setItem(DANMU_SPEED_KEY, clamped.toString()).catch(console.error);
  },
  setDanmuLaneCount: (laneCount) => {
    const clamped = Math.max(4, Math.min(20, laneCount));
    set({ danmuLaneCount: clamped });
    storage
      .setItem(DANMU_LANE_COUNT_KEY, clamped.toString())
      .catch(console.error);
  },
  setDanmuMaxMessages: (maxMessages) => {
    const clamped = Math.max(5, Math.min(200, maxMessages));
    set({ danmuMaxMessages: clamped });
    storage
      .setItem(DANMU_MAX_MESSAGES_KEY, clamped.toString())
      .catch(console.error);
  },
});

// Hydrate danmu settings from storage. Mirrors the app's async load of the
// danmu prefs at store creation (the Zustand store isn't persisted, so the
// toggle/page would show defaults on reload otherwise). Called by the store
// index after the store is created; receives the store instance to avoid a
// circular import.
export function hydrateDanmuSettings(store: {
  setState: (partial: Partial<DanmuSlice>) => void;
}) {
  void (async () => {
    try {
      const [
        storedUnlocked,
        storedEnabled,
        storedOpacity,
        storedSpeed,
        storedLaneCount,
        storedMaxMessages,
      ] = await Promise.all([
        storage.getItem(DANMU_UNLOCKED_KEY),
        storage.getItem(DANMU_ENABLED_KEY),
        storage.getItem(DANMU_OPACITY_KEY),
        storage.getItem(DANMU_SPEED_KEY),
        storage.getItem(DANMU_LANE_COUNT_KEY),
        storage.getItem(DANMU_MAX_MESSAGES_KEY),
      ]);

      const initial = {
        danmuUnlocked: storedUnlocked === "true",
        danmuEnabled: storedEnabled === "true",
        danmuOpacity: 80,
        danmuSpeed: 1,
        danmuLaneCount: 12,
        danmuMaxMessages: 50,
      };
      if (storedOpacity) {
        const parsed = parseInt(storedOpacity);
        if (Number.isFinite(parsed) && parsed >= 0 && parsed <= 100) {
          initial.danmuOpacity = parsed;
        }
      }
      if (storedSpeed) {
        const parsed = parseFloat(storedSpeed);
        if (Number.isFinite(parsed) && parsed >= 0.1 && parsed <= 3) {
          initial.danmuSpeed = parsed;
        }
      }
      if (storedLaneCount) {
        const parsed = parseInt(storedLaneCount);
        if (Number.isFinite(parsed) && parsed >= 4 && parsed <= 20) {
          initial.danmuLaneCount = parsed;
        }
      }
      if (storedMaxMessages) {
        const parsed = parseInt(storedMaxMessages);
        if (Number.isFinite(parsed) && parsed >= 5 && parsed <= 200) {
          initial.danmuMaxMessages = parsed;
        }
      }
      store.setState(initial);
    } catch (error) {
      console.error("Failed to load danmu settings from storage:", error);
    }
  })();
}
