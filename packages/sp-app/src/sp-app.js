import menu from "./menu.js";
import autoUpdater from "./auto-updater";
import { app, BrowserWindow, session, Menu, Tray } from "electron";
import path from "path";
import url from "url";
import pkg from "../package.json";
import { format as urlFormat } from "url";
/* eslint-disable no-console */

app.dock && app.dock.hide();

const imagePath = path.resolve(
  require.resolve("streamplace-ui"),
  "..",
  "images"
);

// Activate the auto-updater unless we're running directly from Electron
if (path.basename(process.argv[0]) !== "Electron") {
  autoUpdater(app);
}

process.on("uncaughtException", err => {
  console.error(err);
  process.exit(1);
});

process.on("unhandledRejection", err => {
  console.error("unhandled rejection");
  console.error(err);
  process.exit(1);
});

let tray = null;
let win;
app.on("ready", () => {
  let iconImage =
    process.platform === "darwin" ? "iconTemplate.png" : "favicon.png";
  tray = new Tray(path.resolve(imagePath, iconImage));
  const contextMenu = Menu.buildFromTemplate([
    { label: `Streamplace v${pkg.version}`, enabled: false },
    { label: "Close Streamplace", role: "quit" }
  ]);
  tray.setToolTip("Streamplace");
  tray.setContextMenu(contextMenu);

  // Login window!
  let url = urlFormat({
    protocol: "file",
    slashes: true,
    pathname: require("path").join(__dirname, "entrypoint.html")
  });

  /////// uncomment from here to have app window show up again /////////

  // win = new BrowserWindow({
  //   width: 350,
  //   height: 450,
  //   title: "Streamplace",
  //   titleBarStyle: "hidden-inset",
  //   show: true
  // });

  // win.loadURL("http://localhost:3939");
});
