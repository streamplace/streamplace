import { app, dialog, ipcMain } from "electron";
import { parseArgs } from "node:util";
import "source-map-support/register";
import { generatePrivateKey, privateKeyToAccount } from "viem/accounts";
import getEnv from "./env";
import makeNode from "./node";
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
        default: "",
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
    let privateKey: `0x${string}`;
    if (process.env.AQD_ADMIN_ACCOUNT_KEY) {
      privateKey = process.env.AQD_ADMIN_ACCOUNT_KEY as `0x${string}`;
    } else {
      privateKey = generatePrivateKey();
    }
    ipcMain.handle("getPrivateKey", () => privateKey);
    const account = privateKeyToAccount(privateKey);
    const env = {
      SP_ADMIN_ACCOUNT: account.address.toLowerCase(),
      SP_ALLOWED_STREAMS: account.address.toLowerCase(),
    };
    if (selfTest) {
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
        await start(env);
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
    console.log(`opening ${startPath}`);
    mainWindow.loadURL(startPath);
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
