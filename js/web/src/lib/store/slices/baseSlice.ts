// StreamKey: a signed-atproto stream-publishing key. Persisted to
// storage so a user returning to the app doesn't have to re-register.
import { StateCreator } from "zustand";
import { storage } from "../../storage";

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

export const createBaseSlice: StateCreator<BaseSlice> = (set) => ({
  hydrated: false,
  hydrate: async () => {
    try {
      // Touch the stored key so we know storage is reachable. We don't
      // load the value into state here because the rest of the store
      // (bluesky slice) owns the actual key lifecycle.
      await storage.getItem(STORED_KEY_KEY);
      set({ hydrated: true });
    } catch {
      set({ hydrated: false });
    }
  },
});
