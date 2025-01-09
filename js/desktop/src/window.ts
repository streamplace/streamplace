import { BrowserWindow, globalShortcut } from "electron";
import getEnv, { MAIN_WINDOW_PRELOAD_WEBPACK_ENTRY } from "./env";
import { resolve } from "path";

export const makeWindow = async (): Promise<BrowserWindow> => {
  const { isDev } = getEnv();
  let logoFile: string;
  if (isDev) {
    // theoretically cwd is aquareum/js/desktop:
    logoFile = resolve(process.cwd(), "assets", "aquareum-logo.png");
  } else {
    logoFile = resolve(process.resourcesPath, "aquareum-logo.png");
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
