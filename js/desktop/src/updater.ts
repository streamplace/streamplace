import electron from "electron";
import ms from "ms";
import {
  IUpdateElectronAppOptions,
  UpdateSourceType,
} from "update-electron-app";
import env from "./env";
import * as build from "./version";

const supportedPlatforms = ["darwin", "win32"];

export default function () {
  initElectronUpdater({
    updateSource: {
      type: UpdateSourceType.StaticStorage,
      baseUrl: `${env().updateBaseUrl}/api/desktop-updates`,
    },
    notifyUser: true,
    updateInterval: "10 minutes",
  });
}

export function initElectronUpdater(opts: IUpdateElectronAppOptions) {
  const { updateSource, updateInterval } = opts;

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
  let feedURL: string;
  let serverType: "default" | "json" = "default";
  const electronVersion = app.getVersion();
  switch (updateSource.type) {
    case UpdateSourceType.StaticStorage: {
      feedURL = `${updateSource.baseUrl}/${platform}/${arch}/${electronVersion}/${build.buildTime}`;

      if (platform === "darwin") {
        feedURL += "/RELEASES.json";
        serverType = "json";
      }
      break;
    }
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

  if (opts.notifyUser) {
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
  }

  // check for updates right away and keep checking later
  autoUpdater.checkForUpdates();
  setInterval(() => {
    autoUpdater.checkForUpdates();
  }, ms(updateInterval));
}
