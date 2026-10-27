import { contextBridge } from "electron";

contextBridge.exposeInMainWorld("SP_ELECTRON", {
  isElectron: true,
});
