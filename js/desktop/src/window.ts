import { BrowserWindow } from "electron";
import { resolve } from "path";
import getEnv from "./env";

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

  // Window-level bindings work under Wayland, where global shortcuts don't.
  window.webContents.on("before-input-event", (event, input) => {
    if (input.type !== "keyDown") {
      return;
    }
    const key = input.key.toLowerCase();
    const devtools =
      (input.shift && (input.control || input.meta) && key === "i") ||
      (input.meta && input.alt && key === "i");
    if (devtools) {
      event.preventDefault();
      window.webContents.toggleDevTools();
      return;
    }
    if (input.shift && (input.control || input.meta) && key === "r") {
      event.preventDefault();
      window.webContents.reload();
    }
  });

  window.removeMenu();

  return window;
};
