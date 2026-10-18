import assert from "node:assert/strict";
import { existsSync } from "node:fs";
import { mkdir, mkdtemp, readdir, readFile, writeFile } from "node:fs/promises";
import os from "node:os";
import { join } from "node:path";
import { test } from "node:test";
import { installPluginDir, pluginState, writeStreamConfig } from "./obs";

// A plugin directory in the Linux/Windows layout package.sh produces.
const makePlugin = async (dir: string, build_time: number) => {
  await mkdir(join(dir, "data"), { recursive: true });
  await mkdir(join(dir, "bin", "64bit"), { recursive: true });
  await writeFile(
    join(dir, "data", "manifest.json"),
    JSON.stringify({ version: `v${build_time}`, build_time }),
  );
  await writeFile(join(dir, "bin", "64bit", "obs-streamplace.so"), "");
};

const setup = async () => {
  const tmp = await mkdtemp(join(os.tmpdir(), "obs-test-"));
  const bundled = join(tmp, "resources", "obs-streamplace");
  await makePlugin(bundled, 2);
  return { bundled, root: join(tmp, "plugins") };
};

test("installs into an empty plugin root", async () => {
  const { bundled, root } = await setup();
  assert.equal((await pluginState(bundled, root)).plugin, "absent");
  assert.equal(await installPluginDir(bundled, root), true);
  assert.deepEqual(await pluginState(bundled, root), {
    plugin: "current",
    bundledVersion: "v2",
    installedVersion: "v2",
  });
  assert.deepEqual(await readdir(root), ["obs-streamplace"]);
});

test("replaces an outdated plugin", async () => {
  const { bundled, root } = await setup();
  const installed = join(root, "obs-streamplace");
  await makePlugin(installed, 1);
  await writeFile(join(installed, "stale"), "");
  assert.equal((await pluginState(bundled, root)).plugin, "outdated");
  assert.equal(await installPluginDir(bundled, root), true);
  assert.equal((await pluginState(bundled, root)).plugin, "current");
  assert.equal(existsSync(join(installed, "stale")), false);
  assert.deepEqual(await readdir(root), ["obs-streamplace"]);
});

test("treats a plugin without a manifest as outdated", async () => {
  const { bundled, root } = await setup();
  await mkdir(join(root, "obs-streamplace", "bin"), { recursive: true });
  assert.deepEqual(await pluginState(bundled, root), {
    plugin: "outdated",
    bundledVersion: "v2",
    installedVersion: undefined,
  });
});

test("leaves a current plugin alone", async () => {
  const { bundled, root } = await setup();
  const installed = join(root, "obs-streamplace");
  await makePlugin(installed, 2);
  await writeFile(join(installed, "marker"), "");
  assert.equal(await installPluginDir(bundled, root), false);
  assert.equal(existsSync(join(installed, "marker")), true);
});

const makeProfiles = async (ini: string, iniBody: string) => {
  const config = await mkdtemp(join(os.tmpdir(), "obs-config-"));
  for (const name of ["Mine", "Other"]) {
    await mkdir(join(config, "basic", "profiles", name), { recursive: true });
    await writeFile(
      join(config, "basic", "profiles", name, "basic.ini"),
      `[General]\nName=${name}\n`,
    );
  }
  await writeFile(
    join(config, "basic", "profiles", "Mine", "service.json"),
    '{"type":"rtmp_custom","settings":{"server":"rtmp://old","key":"old"}}',
  );
  await writeFile(join(config, ini), iniBody);
  return config;
};

const service = async (config: string, profile: string) =>
  JSON.parse(
    await readFile(
      join(config, "basic", "profiles", profile, "service.json"),
      "utf8",
    ),
  );

test("points the current profile at the node", async () => {
  // OBS 31+ keeps the current profile in user.ini, written with a BOM and CRLF.
  const config = await makeProfiles(
    "user.ini",
    "\uFEFF[General]\r\nProfileDir=Other\r\n[Basic]\r\nProfile=Mine\r\nProfileDir=Mine\r\n",
  );
  await writeStreamConfig(config, "http://127.0.0.1:38082", "sk");
  assert.deepEqual(await service(config, "Mine"), {
    type: "rtmp_common",
    settings: {
      service: "Streamplace",
      server: "http://127.0.0.1:38082",
      key: "sk",
    },
  });
  assert.equal(
    await readFile(
      join(config, "basic", "profiles", "Mine", "basic.ini"),
      "utf8",
    ),
    "[General]\nName=Mine\n",
  );
  assert.equal(
    existsSync(join(config, "basic", "profiles", "Other", "service.json")),
    false,
  );
});

test("reads the current profile from global.ini before OBS 31", async () => {
  const config = await makeProfiles(
    "global.ini",
    "[Basic]\nProfileDir=Other\n",
  );
  await writeStreamConfig(config, "http://127.0.0.1:38082", "sk");
  assert.equal((await service(config, "Other")).settings.key, "sk");
});

test("refuses without a profile", async () => {
  const config = await mkdtemp(join(os.tmpdir(), "obs-config-"));
  await assert.rejects(
    writeStreamConfig(config, "http://x", "sk"),
    /no profile/,
  );
});
