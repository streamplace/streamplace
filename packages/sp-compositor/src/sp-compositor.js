const { app, BrowserWindow } = require("electron");
const path = require("path");
const url = require("url");

// Keep a global reference of the window object, if you don't, the window will
// be closed automatically when the JavaScript object is garbage collected.
let win;

const URL = process.env.URL;
if (!URL) {
  throw new Error("No URL Provided!");
}

const options = {
  width: process.env.WIDTH ? parseInt(process.env.WIDTH) : 1920,
  height: process.env.HEIGHT ? parseInt(process.env.HEIGHT) : 1080,
  windowId: `render-${Date.now()}-${Math.floor(Math.random() * 10000000)}`,
  port: process.env.PORT || 64772
};

function createWindow() {
  // Create the browser window.
  win = new BrowserWindow({
    width: options.width,
    height: options.height,
    resizable: false,
    useContentSize: true,
    frame: false,
    // transparent: true,
    title: options.windowId
  });

  win.on("page-title-updated", e => e.preventDefault());

  // and load the index.html of the app.
  win.loadURL(URL);

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
  win = new BrowserWindow({
    width: 1920,
    height: 1080,
    title: "renderer",
    show: false
  });

  win.on("page-title-updated", e => e.preventDefault());

  // and load the index.html of the app.
  win.loadURL(
    url.format({
      pathname: path.join(__dirname, "renderer.html"),
      protocol: "file:",
      slashes: true,
      query: options
    })
  );

  // win.webContents.openDevTools();

  // Emitted when the window is closed.
  win.on("closed", () => {
    // Dereference the window object, usually you would store windows
    // in an array if your app supports multi windows, this is the time
    // when you should delete the corresponding element.
    win = null;
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
