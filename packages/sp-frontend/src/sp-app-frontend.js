import { app, Menu, Tray } from "electron";
import path from "path";

export default function() {
  let tray = null;
  app.on("ready", () => {
    tray = new Tray(
      path.resolve(__dirname, "..", "public", "iconTemplate.png")
    );
    const contextMenu = Menu.buildFromTemplate([
      { label: "Item1", type: "radio" },
      { label: "Item2", type: "radio" },
      { label: "Item3", type: "radio", checked: true },
      { label: "Item4", type: "radio" }
    ]);
    tray.setToolTip("This is my application.");
    tray.setContextMenu(contextMenu);
  });
}
