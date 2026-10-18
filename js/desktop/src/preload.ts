import { contextBridge, ipcRenderer } from "electron";

// Handled in index.ts; see obs.ts for the shapes.
contextBridge.exposeInMainWorld("SP_ELECTRON", {
  isElectron: true,
  obs: {
    status: () => ipcRenderer.invoke("obs:status"),
    installPlugin: () => ipcRenderer.invoke("obs:installPlugin"),
    configureStream: (streamKey: string) =>
      ipcRenderer.invoke("obs:configureStream", streamKey),
  },
});
