import { storage } from "@streamplace/components";
import { v4 as uuidv4 } from "uuid";
import { StateCreator } from "zustand";

export const STORED_KEY_KEY = "storedKey";
export const DID_KEY = "did";
export const DEVICE_ID_KEY = "deviceId";

export interface StreamKey {
  privateKey: string;
  did: string;
  address: string;
}

export interface BaseSlice {
  hydrated: boolean;
  deviceId: string;
  hydrate: () => Promise<void>;
}

export const createBaseSlice: StateCreator<BaseSlice> = (set, get) => ({
  hydrated: false,
  deviceId: "",
  hydrate: async () => {
    try {
      let storedKey: StreamKey | null = null;
      const storedKeyStr = await storage.getItem(STORED_KEY_KEY);
      if (storedKeyStr) {
        storedKey = JSON.parse(storedKeyStr);
      }

      // Load or generate device ID
      let deviceId = await storage.getItem(DEVICE_ID_KEY);
      if (!deviceId) {
        deviceId = uuidv4();
        await storage.setItem(DEVICE_ID_KEY, deviceId);
      }

      set({ hydrated: true, deviceId });
    } catch (e) {
      set({ hydrated: false });
    }
  },
});
