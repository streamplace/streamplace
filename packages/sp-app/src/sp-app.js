import menu from "./menu.js";
import autoUpdater from "./auto-updater";
import SPFrontend from "sp-frontend";
import { app, BrowserWindow, session, Menu } from "electron";
import path from "path";
import url from "url";
/* eslint-disable no-console */

app.dock && app.dock.hide();

// Activate the auto-updater unless we're running directly from Electron
if (path.basename(process.argv[0]) !== "Electron") {
  autoUpdater(app);
}
SPFrontend();

// // Keep a global reference of the window object, if you don"t, the window will
// // be closed automatically when the JavaScript object is garbage collected.
// let win;

// const spUrl = process.env.SP_URL || "stream.place";

// function createWindow() {
//   Menu.setApplicationMenu(menu);

//   // Create the browser window.
//   win = new BrowserWindow({
//     width: 1920,
//     height: 1080,
//     title: "Streamplace",
//     titleBarStyle: "hidden-inset",
//     show: false
//   });

//   win.once("ready-to-show", () => {
//     win.show();
//   });

//   // Little old-school async method here to set all our cookies before loading the page
//   const cookies = [
//     {
//       url: `${spUrl}`,
//       name: "streamplace",
//       value: "w00t",
//       hostOnly: false,
//       expirationDate: 32503680000000 // i've been to the year 3000
//     },
//     {
//       url: `${spUrl}`,
//       name: "appMode",
//       value: "true",
//       hostOnly: false,
//       expirationDate: 32503680000000 // i've been to the year 3000
//     }
//   ];

//   const done = () => {
//     win.loadURL(spUrl);
//   };

//   const setCookie = () => {
//     if (cookies.length === 0) {
//       return done();
//     }
//     const cookie = cookies.pop();
//     win.webContents.session.cookies.set(cookie, error => {
//       if (error) {
//         console.error(error);
//         throw error;
//       }
//       setCookie();
//     });
//   };

//   // For now we're just gonna operate with no cache on load
//   win.webContents.session.clearCache(err => {
//     if (err) {
//       throw err;
//     }
//     setCookie();
//   });

//   // Open the DevTools.
//   // win.webContents.openDevTools()

//   // Emitted when the window is closed.
//   win.on("closed", () => {
//     // Dereference the window object, usually you would store windows
//     // in an array if your app supports multi windows, this is the time
//     // when you should delete the corresponding element.
//     win = null;
//   });
// }

// // This method will be called when Electron has finished
// // initialization and is ready to create browser windows.
// // Some APIs can only be used after this event occurs.
// app.on("ready", createWindow);

// // Quit when all windows are closed.
// app.on("window-all-closed", () => {
//   // On macOS it is common for applications and their menu bar
//   // to stay active until the user quits explicitly with Cmd + Q
//   if (process.platform !== "darwin") {
//     app.quit();
//   }
// });

// app.on("activate", () => {
//   // On macOS it's common to re-create a window in the app when the
//   // dock icon is clicked and there are no other windows open.
//   if (win === null) {
//     createWindow();
//   }
// });
