import { app, Menu, Tray } from "electron";
import path from "path";
import pkg from "../package.json";

export default function() {
  let tray = null;
  app.on("ready", () => {
    let iconImage =
      process.platform === "darwin" ? "iconTemplate.png" : "favicon.png";
    tray = new Tray(path.resolve(__dirname, "..", "public", iconImage));
    const contextMenu = Menu.buildFromTemplate([
      { label: `Streamplace v${pkg.version}`, enabled: false },
      { label: "Close Streamplace", role: "quit" }
    ]);
    tray.setToolTip("Streamplace");
    tray.setContextMenu(contextMenu);
  });
}
