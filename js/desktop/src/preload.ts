import { contextBridge, ipcRenderer } from "electron";

contextBridge.exposeInMainWorld("SP_ELECTRON", {
  isElectron: true,
});
contextBridge.exposeInMainWorld("getPrivateKey", () => {
  return ipcRenderer.invoke("getPrivateKey");
});
