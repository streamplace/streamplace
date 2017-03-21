// Copyright (c) The LHTML team
// See LICENSE for details.

const log = require("electron-log");
const {autoUpdater} = require("electron-updater");

function sendStatusToWindow(text) {
  log.info(text);
}

export default function(app) {
  autoUpdater.on("checking-for-update", () => {
    sendStatusToWindow("Checking for update...");
  });
  autoUpdater.on("update-available", (ev, info) => {
    sendStatusToWindow("Update available.");
  });
  autoUpdater.on("update-not-available", (ev, info) => {
    sendStatusToWindow("Update not available.");
  });
  autoUpdater.on("error", (ev, err) => {
    sendStatusToWindow("Error in auto-updater.");
  });
  autoUpdater.on("download-progress", (ev, progressObj) => {
    sendStatusToWindow("Download progress...");
  });
  autoUpdater.on("update-downloaded", (ev, info) => {
    sendStatusToWindow("Update downloaded; restarting");
    autoUpdater.quitAndInstall();
  });

  app.on("ready", function()  {
    autoUpdater.checkForUpdates();
  });
}
