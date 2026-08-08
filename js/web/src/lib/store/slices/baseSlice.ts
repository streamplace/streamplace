// StreamKey: a signed-atproto stream-publishing key. Persisted to
// storage so a user returning to the app doesn't have to re-register.
import { StateCreator } from "zustand";
import { storage } from "../../storage";
import { AppStore } from "../index";

export const STORED_KEY_KEY = "storedKey";
export const DID_KEY = "did";

export interface StreamKey {
  privateKey: string;
  did: string;
  address: string;
}

export interface BaseSlice {
  hydrated: boolean;
  hydrate: () => Promise<void>;
}

export const createBaseSlice: StateCreator<AppStore, [], [], BaseSlice> = (
  set,
) => ({
  hydrated: false,
  hydrate: async () => {
    try {
      const stored = await storage.getItem(STORED_KEY_KEY);
      if (stored) {
        try {
          const parsed = JSON.parse(stored) as StreamKey;
          set({ storedKey: parsed });
        } catch {
          // Corrupted stored key; remove it so it doesn't cause issues.
          await storage.removeItem(STORED_KEY_KEY);
        }
      }
      set({ hydrated: true });
    } catch {
      set({ hydrated: false });
    }
  },
});
