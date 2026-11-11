import { contextBridge, ipcRenderer } from "electron";

// A login callback can arrive before the renderer subscribes to it, so buffer
// the newest URL until then.
let pendingLoginURL: string | undefined;
const loginCallbacks = new Set<(url: string) => void>();
ipcRenderer.on("auth:callback", (_event, url: string) => {
  if (loginCallbacks.size === 0) {
    pendingLoginURL = url;
    return;
  }
  for (const callback of loginCallbacks) {
    callback(url);
  }
});

// Handled in index.ts; see obs.ts for the shapes.
contextBridge.exposeInMainWorld("SP_ELECTRON", {
  isElectron: true,
  obs: {
    status: () => ipcRenderer.invoke("obs:status"),
    installPlugin: () => ipcRenderer.invoke("obs:installPlugin"),
    configureStream: (streamKey: string) =>
      ipcRenderer.invoke("obs:configureStream", streamKey),
  },
  // Desktop logins run in the system browser so passkeys work; the OS hands
  // the OAuth callback back to the first instance on a custom scheme.
  auth: {
    openLogin: (url: string) => ipcRenderer.invoke("auth:openLogin", url),
    onCallback: (callback: (url: string) => void) => {
      loginCallbacks.add(callback);
      if (pendingLoginURL !== undefined) {
        const url = pendingLoginURL;
        pendingLoginURL = undefined;
        callback(url);
      }
      return () => {
        loginCallbacks.delete(callback);
      };
    },
  },
});
