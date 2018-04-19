import menu from "./menu.js";
import autoUpdater from "./auto-updater";
import { app, BrowserWindow, session, Menu, Tray, ipcMain } from "electron";
import path from "path";
import url from "url";
import pkg from "../package.json";
import { format as urlFormat } from "url";
import SP, { config } from "sp-client";

/* eslint-disable no-console */

const DOMAIN = config.require("DOMAIN");
const PROTOCOL = config.require("PROTOCOL");

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

const IS_DEVELOPMENT = process.env.NODE_ENV === "development";
let iconImage;
if (process.platform === "darwin") {
  if (IS_DEVELOPMENT) {
    iconImage = "iconMacDev.png";
  } else {
    iconImage = "iconTemplate.png";
  }
} else {
  if (IS_DEVELOPMENT) {
    iconImage = "faviconDev.png";
  } else {
    iconImage = "favicon.png";
  }
}

let tray = null;
let win;

const makeTray = ({ profile } = {}) => {
  let versionString = `Streamplace v${pkg.version}`;
  if (process.env.NODE_ENV === "development") {
    versionString += "-dev";
  }
  if (!tray) {
    tray = new Tray(path.resolve(imagePath, iconImage));
  }
  const items = [];
  if (profile) {
    items.push({
      label: `Log out ${profile.name}...`,
      click: () => {
        makeTray();
        loginWindow({ logout: true });
      }
    });
  } else {
    items.push({
      label: `Log in...`,
      click: () => {
        loginWindow({ logout: false });
      }
    });
  }
  items.push(
    { label: versionString, enabled: false },
    { label: "Close Streamplace", role: "quit" }
  );
  const contextMenu = Menu.buildFromTemplate(items);
  tray.setToolTip("Streamplace");
  tray.setContextMenu(contextMenu);
};

app.on("window-all-closed", () => {
  // don't need to do anything, it just quits otherwise lol
});

const loginWindow = ({ logout = false } = {}) => {
  win = new BrowserWindow({
    width: 350,
    height: 450,
    resizable: false,
    title: "Streamplace",
    titleBarStyle: "hidden-inset",
    show: false,
    webPreferences: {
      nodeIntegration: false,
      preload: path.resolve(__dirname, "login-preload.js")
    }
  });

  let base = IS_DEVELOPMENT
    ? "http://localhost:3939"
    : `${PROTOCOL}://${DOMAIN}`;
  win.loadURL(`${base}?electronLogin=true&electronLogout=${logout}`);
};

app.on("ready", () => {
  makeTray();
  let url = urlFormat({
    protocol: "file",
    slashes: true,
    pathname: path.resolve(
      require.resolve("sp-frontend"),
      "..",
      "..",
      "build-electron",
      "index.html"
    )
  });

  ipcMain.on("logged-in", (event, { token, profile }) => {
    win.close();
    SP.connect({ token }).then(() => {
      makeTray({ profile });
    });
  });

  ipcMain.on("not-logged-in", event => {
    console.log("not-logged-in");
    win.show();
  });

  loginWindow();
});
