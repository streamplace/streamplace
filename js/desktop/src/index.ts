import { app, BrowserWindow, dialog, ipcMain } from "electron";
import { parseArgs } from "node:util";
import "source-map-support/register";
import { generatePrivateKey } from "viem/accounts";
import getEnv from "./env";
import makeNode from "./node";
import { configureStream, installPlugin, status as obsStatus } from "./obs";
import runTests, { allTestNames } from "./tests/test-runner";
import initUpdater from "./updater";
import { makeWindow } from "./window";

// Handle creating/removing shortcuts on Windows when installing/uninstalling.
if (require("electron-squirrel-startup")) {
  app.quit();
} else {
  // Electron leaves Chromium switches and the app entry in process.argv.
  // Keep parsing our own options without rejecting those Electron arguments.
  const { values: args, positionals } = parseArgs({
    strict: false,
    allowPositionals: true,
    options: {
      path: {
        type: "string",
        default: "/desktop",
      },
      "self-test": {
        type: "boolean",
      },
      "self-test-duration": {
        type: "string",
        default: "300000",
      },
      "tests-to-run": {
        type: "string",
        default: allTestNames.join(","),
      },
    },
  });
  const path = args.path;
  const testsToRun = args["tests-to-run"];
  const testDuration = args["self-test-duration"];
  const selfTest = args["self-test"];
  if (
    typeof path !== "string" ||
    typeof testsToRun !== "string" ||
    typeof testDuration !== "string" ||
    (selfTest !== undefined && typeof selfTest !== "boolean")
  ) {
    throw new TypeError("Invalid Streamplace Desktop command-line options");
  }
  const env = getEnv();
  console.log(
    "starting with: ",
    JSON.stringify({ args, positionals, env }, null, 2),
  );

  app.on("ready", async () => {
    if (selfTest) {
      // The self-tests stream with a key of their own.
      const privateKey = (process.env.AQD_ADMIN_ACCOUNT_KEY ??
        generatePrivateKey()) as `0x${string}`;
      app.on("window-all-closed", () => {
        // need to override this to prevent the app from quitting
      });
      const success = await runTests(
        testsToRun.split(","),
        testDuration,
        privateKey,
      );
      console.log("tests finished");
      if (!success) {
        app.exit(1);
      } else {
        app.exit(0);
      }
    } else {
      try {
        await start({});
      } catch (e) {
        console.error(e);
        const dialogOpts: Electron.MessageBoxOptions = {
          type: "info",
          buttons: ["Quit Streamplace"],
          title: "Error on Bootup",
          message:
            "Please report to the Streamplace developers at git.stream.place!",
          detail: e.message + "\n" + e.stack,
        };

        await dialog.showMessageBox(dialogOpts);
        app.quit();
      }
    }
  });

  const start = async (env: { [k: string]: string }): Promise<void> => {
    app.on("window-all-closed", () => {
      if (process.platform !== "darwin") {
        app.quit();
      }
    });
    const { skipNode, nodeFrontend, noUpdate } = getEnv();
    if (!noUpdate) {
      initUpdater();
    }
    let loadAddr;
    if (!skipNode) {
      const { addr } = await makeNode({ env, autoQuit: true });
      loadAddr = addr;
    }
    const mainWindow = await makeWindow();

    let startPath;
    if (nodeFrontend) {
      startPath = `${loadAddr}${path}`;
    } else {
      startPath = `http://127.0.0.1:38081${path}`;
    }

    // For the OBS setup screen, through SP_ELECTRON.obs in preload.ts. Only
    // the app's own pages may call it: signing in sends this window to the
    // PDS, whose pages get the same preload.
    const server = loadAddr ?? "http://127.0.0.1:38082";
    const appOrigin = new URL(startPath).origin;
    const handleObs = <A extends unknown[]>(
      channel: string,
      fn: (...args: A) => unknown,
    ) =>
      ipcMain.handle(channel, (e, ...args) => {
        if (e.senderFrame?.origin !== appOrigin) {
          throw new Error(`${channel} is only available to ${appOrigin}`);
        }
        return fn(...(args as A));
      });
    handleObs("obs:status", () => obsStatus());
    handleObs("obs:installPlugin", () => installPlugin());
    handleObs("obs:configureStream", (streamKey: string) =>
      configureStream(server, streamKey),
    );
    console.log(`opening ${startPath}`);
    mainWindow.loadURL(startPath);

    offerObsPlugin(mainWindow).catch((e) =>
      console.error("OBS plugin check failed", e),
    );
  };

  // Offers to install or update the bundled OBS plugin when OBS could load it.
  const offerObsPlugin = async (window: BrowserWindow) => {
    const { obs, plugin, obsRunning } = await obsStatus();
    if (obs !== "installed" || plugin === "current") {
      return;
    }
    const verb = plugin === "outdated" ? "Update" : "Install";
    const { response } = await dialog.showMessageBox(window, {
      type: "question",
      buttons: [verb, "Later"],
      title: "Streamplace OBS plugin",
      message: `${verb} the Streamplace plugin for OBS?`,
      detail:
        obsRunning && process.platform === "win32"
          ? `Close OBS first, then choose ${verb}.`
          : "OBS streams to Streamplace through it.",
    });
    if (response !== 0) {
      return;
    }
    const { ok, error, restartObs } = await installPlugin();
    if (!ok) {
      await dialog.showMessageBox(window, {
        type: "error",
        title: "Streamplace OBS plugin",
        message: "Couldn't install the OBS plugin",
        detail: error,
      });
    } else if (restartObs) {
      await dialog.showMessageBox(window, {
        type: "info",
        title: "Streamplace OBS plugin",
        message: "Restart OBS to load the new plugin.",
      });
    }
  };

  // This method will be called when Electron has finished
  // initialization and is ready to create browser windows.
  // Some APIs can only be used after this event occurs.

  // Quit when all windows are closed, except on macOS. There, it's common
  // for applications and their menu bar to stay active until the user quits
  // explicitly with Cmd + Q.

  app.on("activate", () => {
    // On OS X it's common to re-create a window in the app when the
    // dock icon is clicked and there are no other windows open.
    // if (BrowserWindow.getAllWindows().length === 0) {
    // }
  });

  // In this file you can include the rest of your app's specific main process
  // code. You can also put them in separate files and import them here.
}
