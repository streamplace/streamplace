import { BrowserWindow, globalShortcut } from "electron";
import { resolve } from "path";
import getEnv, { MAIN_WINDOW_PRELOAD_WEBPACK_ENTRY } from "./env";

export const makeWindow = async (): Promise<BrowserWindow> => {
  const { isDev } = getEnv();
  let logoFile: string;
  if (isDev) {
    // theoretically cwd is streamplace/js/desktop:
    logoFile = resolve(process.cwd(), "assets", "streamplace-logo.png");
  } else {
    logoFile = resolve(process.resourcesPath, "streamplace-logo.png");
  }
  const window = new BrowserWindow({
    height: 600,
    width: 800,
    icon: logoFile,
    webPreferences: {
      preload: MAIN_WINDOW_PRELOAD_WEBPACK_ENTRY,
    },
    // titleBarStyle: "hidden",
    // titleBarOverlay: true,
  });

  globalShortcut.register("CommandOrControl+Shift+I", () => {
    window.webContents.toggleDevTools();
  });

  globalShortcut.register("CommandOrControl+Shift+R", () => {
    window.webContents.reload();
  });

  window.removeMenu();

  return window;
};
