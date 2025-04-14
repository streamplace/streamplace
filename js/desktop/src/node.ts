import os from "os";
import { resolve } from "path";
import { access, constants } from "fs/promises";
import { spawn } from "child_process";
import getEnv from "./env";
import { app } from "electron";

const findExe = async (): Promise<string> => {
  const { isDev } = getEnv();
  let fname = "streamplace";
  let exe: string;
  let platform = os.platform() as string;
  let architecture = os.arch() as string;
  if (platform === "win32") {
    platform = "windows";
    fname += ".exe";
  }
  if (architecture === "x64") {
    architecture = "amd64";
  }
  let binfolder = `build-${platform}-${architecture}`;
  if (isDev) {
    // theoretically cwd is streamplace/js/desktop:
    exe = resolve(process.cwd(), "..", "..", binfolder, fname);
  } else {
    exe = resolve(process.resourcesPath, fname);
  }
  try {
    await access(exe, constants.F_OK);
  } catch (e) {
    throw new Error(
      `could not find streamplace node binary at ${exe}: ${e.message}`,
    );
  }
  return exe;
};

export default async function makeNode(opts: {
  env: { [k: string]: string };
  autoQuit: boolean;
}) {
  const exe = await findExe();
  const addr = opts.env.SP_HTTP_ADDR ?? "127.0.0.1:38082";
  const internalAddr = opts.env.SP_HTTP_INTERNAL_ADDR ?? "127.0.0.1:39092";
  const proc = spawn(exe, [], {
    stdio: "inherit",
    env: {
      ...process.env,
      SP_HTTP_ADDR: addr,
      SP_HTTP_INTERNAL_ADDR: internalAddr,
      ...opts.env,
    },
    windowsHide: true,
  });
  await checkService(`http://${addr}/api/healthz`);

  if (opts.autoQuit) {
    app.on("before-quit", () => {
      proc.kill("SIGTERM");
    });
  }
  proc.on("exit", () => {
    console.log("node exited");
    if (opts.autoQuit) {
      app.quit();
    }
  });

  return {
    proc,
    addr: `http://${addr}`,
    internalAddr: `http://${internalAddr}`,
  };
}

const checkService = (
  url: string,
  interval = 300,
  timeout = 10000,
): Promise<void> => {
  let attempts = 0;
  const maxAttempts = timeout / interval;

  return new Promise((resolve, reject) => {
    const intervalId = setInterval(async () => {
      attempts++;

      try {
        const response = await fetch(url);
        if (response.ok) {
          // Response status in the range 200-299
          clearInterval(intervalId);
          resolve();
        }
      } catch (error) {
        // Fetch failed, continue trying
      }

      if (attempts >= maxAttempts) {
        clearInterval(intervalId);
        reject(new Error("streamplace did not boot up in time"));
      }
    }, interval);
  });
};
