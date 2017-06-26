const { app, BrowserWindow } = require("electron");

app.commandLine.appendSwitch("js-flags", "--harmony-sharedarraybuffer");
app.commandLine.appendSwitch("enable-blink-feature", "SharedArrayBuffer");

let win;

app.on("ready", () => {
  // // Create the browser window.
  // win = new BrowserWindow({
  //   width: 1920,
  //   height: 1080,
  //   title: "Streamplace Compositor",
  //   show: true,
  // });
  // win.loadURL("about:blank");
  // // Open the DevTools.
  // // win.webContents.openDevTools()
  // // Emitted when the window is closed.
  // win.on("closed", () => {
  //   // Dereference the window object, usually you would store windows
  //   // in an array if your app supports multi windows, this is the time
  //   // when you should delete the corresponding element.
  //   win = null;
  // });
});
