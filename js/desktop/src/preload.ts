import { contextBridge, ipcRenderer } from "electron";

// Handled in index.ts; see obs.ts for the shapes.
contextBridge.exposeInMainWorld("SP_ELECTRON", {
  isElectron: true,
  app: {
    openLogFolder: () => ipcRenderer.invoke("app:openLogFolder"),
    focus: () => ipcRenderer.invoke("app:focus"),
  },
  obs: {
    status: () => ipcRenderer.invoke("obs:status"),
    installPlugin: () => ipcRenderer.invoke("obs:installPlugin"),
    configureStream: (did: string, streamKey: string) =>
      ipcRenderer.invoke("obs:configureStream", did, streamKey),
  },
  // Desktop logins open in the system browser so passkeys work; the local
  // node holds the callback for the app to poll.
  auth: {
    openLogin: (url: string) => ipcRenderer.invoke("auth:openLogin", url),
  },
});
