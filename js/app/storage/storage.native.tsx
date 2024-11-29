import Storage from "expo-sqlite/kv-store";
import { AQStorage } from "./storage.shared";

export default class NativeStorage implements AQStorage {
  async getItem(key: string): Promise<string | null> {
    const value = await Storage.getItem(key);
    return value ?? null;
  }

  async setItem(key: string, value: string): Promise<void> {
    await Storage.setItem(key, value);
  }

  async removeItem(key: string): Promise<void> {
    await Storage.removeItem(key);
  }
}
