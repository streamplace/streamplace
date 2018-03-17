// Copyright (c) The LHTML team
// See LICENSE for details.

import log from "electron-log";
import { autoUpdater } from "electron-updater";
import AutoLaunch from "auto-launch";
const IS_LOCAL_DEV = !!process.env.SP_LOCAL_DEV;

const UPDATE_INTERVAL = 1000 * 60 * 5; // 5 min

const streamplaceAutoLauncher = new AutoLaunch({
  name: "Streamplace",
  isHidden: true
});

streamplaceAutoLauncher
  .isEnabled()
  .then(function(isEnabled) {
    if (isEnabled) {
      return;
    }
    if (IS_LOCAL_DEV) {
      log.info("not enabling autolaunch in dev");
      return;
    }
    log.info("enabling autolaunch");
    streamplaceAutoLauncher.enable();
  })
  .catch(err => {
    log.error(err);
  });

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

  app.on("ready", function() {
    if (IS_LOCAL_DEV) {
      log.info("not running autoupdate in dev");
      return;
    }
    autoUpdater.checkForUpdates();
    setInterval(() => {
      autoUpdater.checkForUpdates();
    }, UPDATE_INTERVAL);
  });
}
