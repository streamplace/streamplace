import { contextBridge, ipcRenderer } from "electron";

contextBridge.exposeInMainWorld("AQ_ELECTRON", {
  isElectron: true,
});
contextBridge.exposeInMainWorld("getPrivateKey", () => {
  return ipcRenderer.invoke("getPrivateKey");
});
