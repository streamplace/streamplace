import { spawn } from "child_process";
import { app, dialog, shell } from "electron";
import { access, constants, mkdir, open, rename, rm } from "fs/promises";
import os from "os";
import { resolve } from "path";
import { setTimeout as delay } from "timers/promises";
import getEnv from "./env";

export const getLogDir = (): string => app.getPath("logs");

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
    throw new Error(`could not find streamplace node binary at ${exe}`, {
      cause: e,
    });
  }
  return exe;
};

export default async function makeNode(opts: {
  env: { [k: string]: string };
  autoQuit: boolean;
}) {
  const exe = await findExe();
  const addr =
    opts.env.SP_HTTP_ADDR ?? process.env.SP_HTTP_ADDR ?? "127.0.0.1:38080";
  const internalAddr =
    opts.env.SP_HTTP_INTERNAL_ADDR ??
    process.env.SP_HTTP_INTERNAL_ADDR ??
    "127.0.0.1:39090";
  // A single-user node: it indexes only the accounts logged into it, signs
  // them in and pushes their streams through the station, and takes their
  // firehose events from per-DID tracks on the station's MoQ relay. An empty
  // SP_UPSTREAM_STATION runs an ordinary standalone node instead.
  const upstreamStation =
    opts.env.SP_UPSTREAM_STATION ??
    process.env.SP_UPSTREAM_STATION ??
    "did:web:stream.place";
  const relayHost =
    opts.env.SP_RELAY_HOST ??
    process.env.SP_RELAY_HOST ??
    (upstreamStation ? "moqt://streamplace.network" : undefined);
  const logDir = getLogDir();
  await mkdir(logDir, { recursive: true });
  const logPath = resolve(logDir, "node.log");
  const previousLogPath = resolve(logDir, "node.previous.log");
  try {
    await access(logPath);
    await rm(previousLogPath, { force: true });
    await rename(logPath, previousLogPath);
  } catch (error) {
    if ((error as NodeJS.ErrnoException).code !== "ENOENT") throw error;
  }
  // Owner-only: node logs can carry session and ingest details.
  const log = (await open(logPath, "w", 0o600)).createWriteStream();
  log.on("error", (error) => console.error("Writing node log failed", error));
  const proc = spawn(exe, [], {
    stdio: ["ignore", "pipe", "pipe"],
    env: {
      ...process.env,
      SP_UPSTREAM_STATION: upstreamStation,
      ...(relayHost !== undefined && { SP_RELAY_HOST: relayHost }),
      SP_HTTP_ADDR: addr,
      SP_HTTP_INTERNAL_ADDR: internalAddr,
      // Every listener on loopback by default: the node takes any valid
      // stream key, so it must only hear this machine's encoders.
      SP_RTMP_ADDR: process.env.SP_RTMP_ADDR ?? "127.0.0.1:1935",
      SP_MOQ_ADDR: process.env.SP_MOQ_ADDR ?? "127.0.0.1:38443",
      ...opts.env,
    },
    windowsHide: true,
  });
  proc.stdout.pipe(process.stdout, { end: false });
  proc.stderr.pipe(process.stderr, { end: false });
  proc.stdout.pipe(log, { end: false });
  proc.stderr.pipe(log, { end: false });

  const boot = new AbortController();
  let healthy = false;
  let quitting = false;
  const shutdown = () => {
    quitting = true;
    proc.kill("SIGTERM");
  };
  if (opts.autoQuit) app.once("before-quit", shutdown);
  proc.once("error", (error) => boot.abort(error));
  proc.once("exit", (code, signal) => {
    if (!healthy) {
      boot.abort(
        new Error(
          `Streamplace node exited before becoming healthy (exit code ${code}, signal ${signal ?? "none"}).`,
        ),
      );
    }
  });
  proc.once("close", async (code, signal) => {
    log.end();
    app.removeListener("before-quit", shutdown);
    if (!healthy || quitting || !opts.autoQuit) return;
    try {
      const { response } = await dialog.showMessageBox({
        type: "error",
        title: "Streamplace node stopped",
        message: "The Streamplace node stopped unexpectedly.",
        detail: `Exit code: ${code}. Signal: ${signal ?? "none"}.`,
        buttons: ["Open log folder", "Quit"],
        cancelId: 1,
      });
      if (response === 0) {
        const error = await shell.openPath(getLogDir());
        if (error) throw new Error(error);
      }
    } catch (error) {
      console.error(error);
    } finally {
      app.quit();
    }
  });
  try {
    await checkService(`http://${addr}/api/healthz`, boot.signal);
    healthy = true;
  } catch (error) {
    shutdown();
    throw error;
  }

  return {
    proc,
    addr: `http://${addr}`,
    internalAddr: `http://${internalAddr}`,
  };
}

const checkService = async (
  url: string,
  signal: AbortSignal,
  interval = 300,
  timeout = 60000,
): Promise<void> => {
  const deadline = AbortSignal.timeout(timeout);
  const abort = AbortSignal.any([signal, deadline]);
  while (!abort.aborted) {
    try {
      const response = await fetch(url, { signal: abort });
      if (response.ok) return;
    } catch {
      // Keep polling until the node exits or startup times out.
    }
    try {
      await delay(interval, undefined, { signal: abort });
    } catch {
      break;
    }
  }
  if (signal.aborted) throw signal.reason;
  throw new Error("streamplace did not boot up in time");
};
