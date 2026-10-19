import { execFile } from "child_process";
import {
  access,
  cp,
  mkdir,
  mkdtemp,
  readFile,
  rename,
  rm,
  writeFile,
} from "fs/promises";
import os from "os";
import { basename, delimiter, join, resolve } from "path";
import { promisify } from "util";
import getEnv from "./env";

const run = promisify(execFile);

export type ObsStatus = {
  obs: "absent" | "installed" | "flatpak";
  plugin: "absent" | "outdated" | "current";
  obsRunning: boolean;
  bundledVersion: string;
  installedVersion?: string;
};

// Flatpak OBS only loads plugins packaged as Flatpak extensions.
const FLATPAK_ERROR =
  "OBS is installed as a Flatpak, which can't load the Streamplace plugin. Install OBS from your distribution or obsproject.com instead.";

const exists = (path: string) =>
  access(path).then(
    () => true,
    () => false,
  );

// Where OBS keeps its settings and profiles.
const obsConfigDir = () => {
  const home = os.homedir();
  if (process.platform === "win32") {
    return join(
      process.env.APPDATA ?? join(home, "AppData", "Roaming"),
      "obs-studio",
    );
  }
  if (process.platform === "darwin") {
    return join(home, "Library", "Application Support", "obs-studio");
  }
  return join(
    process.env.XDG_CONFIG_HOME || join(home, ".config"),
    "obs-studio",
  );
};

// Where OBS 30.2+ loads third-party plugins from; machine-wide on Windows.
const pluginRoot = () => {
  if (process.platform === "win32") {
    return join(
      process.env.ProgramData ?? "C:\\ProgramData",
      "obs-studio",
      "plugins",
    );
  }
  return join(obsConfigDir(), "plugins");
};

// The plugin forge.config.ts unpacks into the app's resources.
const bundledPlugin = () => {
  const name =
    process.platform === "darwin"
      ? "obs-streamplace.plugin"
      : "obs-streamplace";
  if (getEnv().isDev) {
    const platform =
      process.platform === "win32" ? "windows" : process.platform;
    const arch = process.arch === "x64" ? "amd64" : process.arch;
    return resolve(
      process.cwd(),
      "out",
      "obs-plugin",
      `${platform}-${arch}`,
      name,
    );
  }
  return resolve(process.resourcesPath, name);
};

// Written by obs-plugin/package.sh.
const readManifest = async (
  plugin: string,
): Promise<{ version: string; build_time: number } | undefined> => {
  const file = plugin.endsWith(".plugin")
    ? join(plugin, "Contents", "Resources", "manifest.json")
    : join(plugin, "data", "manifest.json");
  try {
    return JSON.parse(await readFile(file, "utf8"));
  } catch {
    return undefined;
  }
};

// How the plugin installed under root compares to the bundled one. A copy
// without a manifest predates them, so it's outdated.
export const pluginState = async (bundled: string, root: string) => {
  const want = await readManifest(bundled);
  if (!want) {
    throw new Error(`no OBS plugin bundled at ${bundled}`);
  }
  const installed = join(root, basename(bundled));
  if (!(await exists(installed))) {
    return { plugin: "absent" as const, bundledVersion: want.version };
  }
  const have = await readManifest(installed);
  return {
    plugin:
      have && have.build_time >= want.build_time
        ? ("current" as const)
        : ("outdated" as const),
    bundledVersion: want.version,
    installedVersion: have?.version,
  };
};

// Copies the bundled plugin into root unless it's already current, replacing
// an older copy. Returns whether it installed anything.
export const installPluginDir = async (
  bundled: string,
  root: string,
): Promise<boolean> => {
  if ((await pluginState(bundled, root)).plugin === "current") {
    return false;
  }
  await mkdir(root, { recursive: true });
  // Staged inside root, so swapping it in is a rename on one filesystem. OBS
  // doesn't load from here: the module sits one directory too deep.
  const stage = await mkdtemp(join(root, ".obs-streamplace-"));
  const target = join(root, basename(bundled));
  const old = join(stage, "old");
  let movedOld = false;
  try {
    await cp(bundled, join(stage, "new"), { recursive: true });
    // On Windows this fails while OBS has the DLL loaded, before anything changes.
    await rename(target, old).then(
      () => (movedOld = true),
      (e) => {
        if (e.code !== "ENOENT") throw e;
      },
    );
    await rename(join(stage, "new"), target).catch(async (e) => {
      // Put the working copy back rather than leave OBS without one.
      if (movedOld) await rename(old, target);
      throw e;
    });
  } finally {
    await rm(stage, { recursive: true, force: true });
  }
  return true;
};

// The profile OBS opens with: user.ini since OBS 31, global.ini before.
const currentProfileDir = async (configDir: string) => {
  for (const file of ["user.ini", "global.ini"]) {
    const text = await readFile(join(configDir, file), "utf8").catch(() => "");
    let section = "";
    for (const line of text.replace(/^\uFEFF/, "").split(/\r?\n/)) {
      const l = line.trim();
      if (l.startsWith("[")) {
        section = l;
      } else if (section === "[Basic]" && l.startsWith("ProfileDir=")) {
        return l.slice("ProfileDir=".length);
      }
    }
  }
  return undefined;
};

// Points OBS's current profile at server with this stream key, leaving the
// profile's other settings alone.
export const writeStreamConfig = async (
  configDir: string,
  server: string,
  key: string,
) => {
  const profile = await currentProfileDir(configDir);
  if (!profile) {
    throw new Error(
      "OBS has no profile yet. Open OBS once, close it, and try again.",
    );
  }
  const service = {
    type: "rtmp_common",
    settings: { service: "Streamplace", server, key },
  };
  await writeFile(
    join(configDir, "basic", "profiles", profile, "service.json"),
    JSON.stringify(service, null, 4),
  );
};

const detectObs = async (): Promise<ObsStatus["obs"]> => {
  if (process.platform === "win32") {
    if (
      await exists(
        join(process.env.ProgramFiles ?? "C:\\Program Files", "obs-studio"),
      )
    ) {
      return "installed";
    }
    return run(
      "reg",
      [
        "query",
        "HKLM\\SOFTWARE\\Microsoft\\Windows\\CurrentVersion\\Uninstall\\OBS Studio",
      ],
      { windowsHide: true },
    ).then(
      () => "installed",
      () => "absent",
    );
  }
  if (process.platform === "darwin") {
    return (await exists("/Applications/OBS.app")) ? "installed" : "absent";
  }
  for (const dir of (process.env.PATH ?? "").split(delimiter)) {
    if (dir && (await exists(join(dir, "obs")))) {
      return "installed";
    }
  }
  if (
    await exists(join(os.homedir(), ".var", "app", "com.obsproject.Studio"))
  ) {
    return "flatpak";
  }
  return (await exists(obsConfigDir())) ? "installed" : "absent";
};

const isObsRunning = async () => {
  if (process.platform === "win32") {
    const { stdout } = await run(
      "tasklist",
      ["/FI", "IMAGENAME eq obs64.exe", "/NH"],
      { windowsHide: true },
    );
    return stdout.includes("obs64.exe");
  }
  return run("pgrep", [
    "-x",
    process.platform === "darwin" ? "OBS" : "obs",
  ]).then(
    () => true,
    () => false,
  );
};

export const status = async (): Promise<ObsStatus> => {
  const [obs, obsRunning, state] = await Promise.all([
    detectObs(),
    isObsRunning(),
    pluginState(bundledPlugin(), pluginRoot()),
  ]);
  return { obs, obsRunning, ...state };
};

export const installPlugin = async (): Promise<{
  ok: boolean;
  error?: string;
  restartObs: boolean;
}> => {
  const [obs, obsRunning] = await Promise.all([detectObs(), isObsRunning()]);
  if (obs !== "installed") {
    const error = obs === "flatpak" ? FLATPAK_ERROR : "OBS isn't installed.";
    return { ok: false, error, restartObs: false };
  }
  if (obsRunning && process.platform === "win32") {
    return {
      ok: false,
      error: "Close OBS, then install the plugin.",
      restartObs: false,
    };
  }
  try {
    const installed = await installPluginDir(bundledPlugin(), pluginRoot());
    return { ok: true, restartObs: installed && obsRunning };
  } catch (e) {
    return { ok: false, error: e.message, restartObs: false };
  }
};

export const configureStream = async (
  server: string,
  streamKey: string,
): Promise<{ ok: boolean; error?: string }> => {
  const [obs, obsRunning] = await Promise.all([detectObs(), isObsRunning()]);
  if (obs !== "installed") {
    return {
      ok: false,
      error: obs === "flatpak" ? FLATPAK_ERROR : "OBS isn't installed.",
    };
  }
  // OBS writes its settings back out when it exits, undoing ours.
  if (obsRunning) {
    return { ok: false, error: "Close OBS, then set up the stream." };
  }
  try {
    await writeStreamConfig(obsConfigDir(), server, streamKey);
    return { ok: true };
  } catch (e) {
    return { ok: false, error: e.message };
  }
};
