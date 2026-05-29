import "@formatjs/intl-displaynames/polyfill";
import "@formatjs/intl-getcanonicallocales/polyfill";
import "@formatjs/intl-locale/polyfill";
import "expo-crypto";
import sqliteStorage from "expo-sqlite/kv-store";
import { MMKV } from "react-native-mmkv";

// Import data for the languages you need
import "@formatjs/intl-displaynames/locale-data/en";

const ATPROTO_OAUTH_MMKV_NAME = "expo-atproto-oauth-client.session";
const OLD_SQLITE_STORE_PREFIX = "@@atproto/oauth-client-react-native:session:";

global.AbortSignal.timeout = function (ms) {
  const controller = new AbortController();

  setTimeout(() => {
    controller.abort(new Error(`signal timed out after ${ms} ms`));
  }, ms);

  return controller.signal;
};
global.AbortSignal.prototype.throwIfAborted = function () {
  const { aborted, reason = "Aborted" } = this;
  if (!aborted) return;
  throw reason;
};
const allKeys = sqliteStorage.getAllKeysSync();

function migrateOAuthStorage() {
  const mmkvStorage = new MMKV({
    id: ATPROTO_OAUTH_MMKV_NAME,
  });

  for (const key of allKeys) {
    if (!key.startsWith(OLD_SQLITE_STORE_PREFIX)) {
      continue;
    }
    const did = key.split(OLD_SQLITE_STORE_PREFIX)[1];
    const oldValue = sqliteStorage.getItemSync(key);
    if (!oldValue) {
      console.warn(`[oauth-migration] no value found for ${key}, skipping`);
      continue;
    }
    const newValue = mmkvStorage.getString(did);
    if (newValue) {
      console.warn(
        `[oauth-migration] looks like we're already logged in on mmkv, no need to migrate ${did}`,
      );
      continue;
    }
    console.log(`[oauth-migration] migrating ${did}`);
    mmkvStorage.set(did, oldValue);
    sqliteStorage.removeItem(key);
  }
}

try {
  migrateOAuthStorage();
} catch (error) {
  console.error(`[oauth-migration] error migrating storage`, error);
}
