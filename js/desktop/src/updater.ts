import electron from "electron";
import env from "./env";
import * as build from "./version";

const supportedPlatforms = ["darwin", "win32"];

export default function () {
  // exit early on unsupported platforms, e.g. `linux`
  if (!supportedPlatforms.includes(process?.platform)) {
    log(
      `Electron's autoUpdater does not support the '${process.platform}' platform. Ref: https://www.electronjs.org/docs/latest/api/auto-updater#platform-notices`,
    );
    return;
  }

  const { app, autoUpdater, dialog } = electron;
  let platform: string = process.platform;
  let arch: string = process.arch;
  if (platform === "win32") {
    platform = "windows";
  }
  if (arch === "x64") {
    arch = "amd64";
  }
  let feedURL = `${env().updateBaseUrl}/api/desktop-updates/${platform}/${arch}/${app.getVersion()}/${build.buildTime}`;
  let serverType: "default" | "json" = "default";
  if (platform === "darwin") {
    feedURL += "/RELEASES.json";
    serverType = "json";
  }

  const requestHeaders = {
    "User-Agent": `streamplace-desktop/${build.version}`,
  };

  function log(...args: any[]) {
    console.log(...args);
  }

  log("feedURL", feedURL);
  log("requestHeaders", requestHeaders);
  autoUpdater.setFeedURL({
    url: feedURL,
    headers: requestHeaders,
    serverType,
  });

  autoUpdater.on("error", (err) => {
    log("updater error");
    log(err);
  });

  autoUpdater.on("checking-for-update", () => {
    log("checking-for-update");
  });

  autoUpdater.on("update-available", () => {
    log("update-available; downloading...");
  });

  autoUpdater.on("update-not-available", () => {
    log("update-not-available");
  });

  autoUpdater.on(
    "update-downloaded",
    (event, releaseNotes, releaseName, releaseDate, updateURL) => {
      log("update-downloaded", [
        event,
        releaseNotes,
        releaseName,
        releaseDate,
        updateURL,
      ]);

      const dialogOpts: Electron.MessageBoxOptions = {
        type: "info",
        buttons: ["Restart", "Later"],
        title: "Application Update",
        message: platform === "windows" ? releaseNotes : releaseName,
        detail:
          "A new version has been downloaded. Restart the application to apply the updates.",
      };

      dialog.showMessageBox(dialogOpts).then(({ response }) => {
        if (response === 0) autoUpdater.quitAndInstall();
      });
    },
  );

  // check for updates right away and keep checking later
  autoUpdater.checkForUpdates();
  setInterval(
    () => {
      autoUpdater.checkForUpdates();
    },
    10 * 60 * 1000,
  );
}
