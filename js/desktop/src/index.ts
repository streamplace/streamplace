import { app, dialog, ipcMain } from "electron";
import { parseArgs } from "node:util";
import "source-map-support/register";
import getEnv from "./env";
import makeNode from "./node";
import initUpdater from "./updater";
import { generatePrivateKey, privateKeyToAccount } from "viem/accounts";
import { makeWindow } from "./window";
import runTests from "./tests/test-runner";
import { allTestNames } from "./tests/test-runner";

// Handle creating/removing shortcuts on Windows when installing/uninstalling.
if (require("electron-squirrel-startup")) {
  app.quit();
} else {
  const { values: args, positionals } = parseArgs({
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
    if (args["self-test"]) {
      const success = await runTests(
        args["tests-to-run"].split(","),
        args["self-test-duration"],
        privateKey,
      );
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
    initUpdater();
    const { skipNode, nodeFrontend } = getEnv();
    let loadAddr;
    if (!skipNode) {
      const { addr } = await makeNode({ env, autoQuit: true });
      loadAddr = addr;
    }
    const mainWindow = await makeWindow();

    let startPath;
    if (nodeFrontend) {
      startPath = `${loadAddr}${args.path}`;
    } else {
      startPath = `http://127.0.0.1:38081${args.path}`;
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
  // app.on("window-all-closed", () => {
  //   if (process.platform !== "darwin") {
  //     app.quit();
  //   }
  // });

  app.on("activate", () => {
    // On OS X it's common to re-create a window in the app when the
    // dock icon is clicked and there are no other windows open.
    // if (BrowserWindow.getAllWindows().length === 0) {
    // }
  });

  // In this file you can include the rest of your app's specific main process
  // code. You can also put them in separate files and import them here.
}
