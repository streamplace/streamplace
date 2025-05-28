import Storage from "expo-sqlite/kv-store";
import { AQStorage } from "./storage.shared";
import { Lock } from "./lock";

// Needed because concurrent calls seem to return with a locked database
const lock = new Lock();

export default class NativeStorage implements AQStorage {
  async getItem(key: string): Promise<string | null> {
    return lock.critical(async () => {
      try {
        const value = await Storage.getItem(key);
        return value ?? null;
      } catch (e) {
        console.error(`error in NativeStorage.getItem: ${e}`);
        throw e;
      }
    });
  }

  async setItem(key: string, value: string): Promise<void> {
    return lock.critical(async () => {
      try {
        await Storage.setItem(key, value);
      } catch (e) {
        console.error(`error in NativeStorage.setItem: ${e}`);
        throw e;
      }
    });
  }

  async removeItem(key: string): Promise<void> {
    return lock.critical(async () => {
      try {
        await Storage.removeItem(key);
      } catch (e) {
        console.error(`error in NativeStorage.removeItem: ${e}`);
        throw e;
      }
    });
  }
}
