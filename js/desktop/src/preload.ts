import { contextBridge, ipcRenderer } from "electron";

let pendingObsRequest = false;
const obsCallbacks = new Set<() => void>();
ipcRenderer.on("obs:keyRequest", () => {
  if (obsCallbacks.size === 0) pendingObsRequest = true;
  for (const callback of obsCallbacks) callback();
});

// Handled in index.ts; see obs.ts for the shapes.
contextBridge.exposeInMainWorld("SP_ELECTRON", {
  isElectron: true,
  app: {
    openLogFolder: () => ipcRenderer.invoke("app:openLogFolder"),
  },
  obs: {
    status: () => ipcRenderer.invoke("obs:status"),
    installPlugin: () => ipcRenderer.invoke("obs:installPlugin"),
    configureStream: (did: string, streamKey: string) =>
      ipcRenderer.invoke("obs:configureStream", did, streamKey),
    onKeyRequest: (callback: () => void) => {
      obsCallbacks.add(callback);
      if (pendingObsRequest) {
        pendingObsRequest = false;
        callback();
      }
      return () => {
        obsCallbacks.delete(callback);
      };
    },
  },
  // Desktop logins open in the system browser so passkeys work; the local
  // node holds the callback for the app to poll.
  auth: {
    openLogin: (url: string) => ipcRenderer.invoke("auth:openLogin", url),
  },
});
