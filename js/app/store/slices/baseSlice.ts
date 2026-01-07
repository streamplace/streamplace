import { storage } from "@streamplace/components";
import { StateCreator } from "zustand";

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

export const createBaseSlice: StateCreator<BaseSlice> = (set, get) => ({
  hydrated: false,
  hydrate: async () => {
    try {
      let storedKey: StreamKey | null = null;
      const storedKeyStr = await storage.getItem(STORED_KEY_KEY);
      if (storedKeyStr) {
        storedKey = JSON.parse(storedKeyStr);
      }
      set({ hydrated: true });
    } catch (e) {
      set({ hydrated: false });
    }
  },
});
