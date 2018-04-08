const { app, BrowserWindow } = require("electron");
const path = require("path");
const url = require("url");
const debug = require("debug");
const log = debug("sp:sp-compositor");

// Keep a global reference of the window object, if you don't, the window will
// be closed automatically when the JavaScript object is garbage collected.
let win;
/* eslint-disable no-console */
const usage = () => {
  console.error(
    "usage: docker run stream.place/sp-compositor [website] <rtmp url>"
  );
};

const WEBSITE = process.argv[3] || process.env.WEBSITE;
const RTMP = process.argv[4] || process.env.RTMP;
if (!WEBSITE) {
  usage();
  throw new Error("No WEBSITE Provided!");
}

const options = {
  width: process.env.WIDTH ? parseInt(process.env.WIDTH) : 1920,
  height: process.env.HEIGHT ? parseInt(process.env.HEIGHT) : 1080,
  windowId: `render-${Date.now()}-${Math.floor(Math.random() * 10000000)}`,
  port: process.env.PORT || 64772
};

if (RTMP) {
  options.rtmp = RTMP;
  log("outputting to rtmp");
} else {
  log(`outputting to port ${options.port}`);
}

setInterval(() => {
  debug(`ping ${Date.now()}`);
}, 1000);

function createWindow() {
  // Create the browser window.
  win = new BrowserWindow({
    width: options.width,
    height: options.height,
    resizable: false,
    useContentSize: true,
    frame: false,
    // transparent: true,
    title: options.windowId,
    webPreferences: {
      nodeIntegration: false
    }
  });

  win.on("page-title-updated", e => e.preventDefault());

  // and load the index.html of the app.
  win.loadURL(WEBSITE);

  // Emitted when the window is closed.
  win.on("closed", () => {
    // Dereference the window object, usually you would store windows
    // in an array if your app supports multi windows, this is the time
    // when you should delete the corresponding element.
    win = null;
  });
}

let captureWin;
function createCapture() {
  // Create the browser window.
  captureWin = new BrowserWindow({
    width: 1920,
    height: 1080,
    title: "renderer",
    show: false,
    webPreferences: {
      preload: path.resolve(__dirname, "preload.js")
    }
  });

  captureWin.on("page-title-updated", e => {
    debug("page-title-updated", e);
    e.preventDefault();
  });

  // and load the index.html of the app.
  const rendererUrl = url.format({
    pathname: path.join(__dirname, "renderer.html"),
    protocol: "file:",
    slashes: true,
    query: options
  });
  log(`loading ${rendererUrl}`);
  captureWin.loadURL(rendererUrl);

  // win.webContents.openDevTools();

  // Emitted when the window is closed.
  captureWin.on("closed", () => {
    // Dereference the window object, usually you would store windows
    // in an array if your app supports multi windows, this is the time
    // when you should delete the corresponding element.
    captureWin = null;
  });
}

// This method will be called when Electron has finished
// initialization and is ready to create browser windows.
// Some APIs can only be used after this event occurs.
app.on("ready", () => {
  createWindow();
  createCapture();
});

// Quit when all windows are closed.
app.on("window-all-closed", () => {
  // On macOS it is common for applications and their menu bar
  // to stay active until the user quits explicitly with Cmd + Q
  if (process.platform !== "darwin") {
    app.quit();
  }
});

app.on("activate", () => {
  // On macOS it's common to re-create a window in the app when the
  // dock icon is clicked and there are no other windows open.
  if (win === null) {
    createWindow();
  }
});

// In this file you can include the rest of your app's specific main process
// code. You can also put them in separate files and require them here.
