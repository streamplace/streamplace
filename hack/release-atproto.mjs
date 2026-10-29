#!/usr/bin/env node
// Artifact staging belongs to the producer graph, not the standalone publisher.
import { spawn } from "node:child_process";
import { createHash } from "node:crypto";
import { createReadStream, createWriteStream } from "node:fs";
import {
  copyFile,
  lstat,
  mkdir,
  mkdtemp,
  readdir,
  readFile,
  rm,
} from "node:fs/promises";
import { tmpdir } from "node:os";
import path from "node:path";
import { Readable } from "node:stream";
import { pipeline } from "node:stream/promises";
import {
  isReleaseBuildHelper,
  releaseProjectForFile,
} from "../js/streamplace/scripts/release-lib.mjs";

const [mode, directory, version, commit, channel, changelog, dryRun, macHost] =
  process.argv.slice(2);
const nodePrefix = `streamplace-${version}-`;
const desktopPrefix = `streamplace-desktop-${version}-`;
const required = [
  ...["linux-amd64", "linux-arm64", "darwin-amd64", "darwin-arm64"].map(
    (target) => `${nodePrefix}${target}.tar.gz`,
  ),
  `${nodePrefix}windows-amd64.zip`,
  ...["amd64", "arm64"].map((arch) => `${nodePrefix}linux-${arch}.deb`),
  ...["debug", "release"].flatMap((flavor) =>
    ["apk", "aab"].map((ext) => `${nodePrefix}android-${flavor}.${ext}`),
  ),
  ...["amd64", "arm64"].map((arch) => `${desktopPrefix}linux-${arch}.AppImage`),
  `${desktopPrefix}windows-amd64.exe`,
  ...["amd64", "arm64"].flatMap((arch) =>
    ["zip", "dmg"].map((ext) => `${desktopPrefix}darwin-${arch}.${ext}`),
  ),
];
const ios = `${nodePrefix}ios-release.xcarchive.tar.gz`;
const apks = ["debug", "release"].map(
  (flavor) => `${nodePrefix}android-${flavor}.apks`,
);
const sidecar = `${desktopPrefix}windows-amd64.nupkg.sha1`;
const known = new Set([...required, ...apks, ios]);
const isNupkg = (name) =>
  name.startsWith(`${desktopPrefix}windows-amd64.`) &&
  /^[0-9a-f]{40}\.nupkg$/.test(
    name.slice(`${desktopPrefix}windows-amd64.`.length),
  );
const isHelper = (name) => isReleaseBuildHelper(name, version);
const isMatching = (name) => Boolean(releaseProjectForFile(name, version));
const fail = (message) => {
  throw new Error(message);
};

async function digest(file, algorithm) {
  const hash = createHash(algorithm);
  for await (const chunk of createReadStream(file)) hash.update(chunk);
  return hash.digest("hex");
}

// Never forward JOB-TOKEN across origins (including object-storage redirects),
// nor downgrade an HTTPS registry connection. No curl -L with custom headers.
async function request(url) {
  let current = new URL(url);
  const origin = current.origin;
  let authenticated = true;
  for (let redirects = 0; redirects <= 5; redirects++) {
    const response = await fetch(current, {
      redirect: "manual",
      headers: authenticated ? { "JOB-TOKEN": process.env.CI_JOB_TOKEN } : {},
    });
    if ([301, 302, 303, 307, 308].includes(response.status)) {
      const location = response.headers.get("location");
      await response.body?.cancel();
      if (!location) fail("registry redirect has no location");
      const next = new URL(location, current);
      if (
        !["http:", "https:"].includes(next.protocol) ||
        next.username ||
        next.password ||
        (current.protocol === "https:" && next.protocol !== "https:")
      )
        fail("unsafe registry redirect");
      if (next.origin !== origin) authenticated = false;
      current = next;
      continue;
    }
    if (!response.ok) {
      await response.body?.cancel();
      fail(`GitLab registry request failed (HTTP ${response.status})`);
    }
    return response;
  }
  fail("too many registry redirects");
}

async function list(url) {
  const values = [];
  for (let page = 1; ; page++) {
    const current = new URL(url);
    current.searchParams.set("per_page", "100");
    current.searchParams.set("page", String(page));
    const response = await request(current);
    const batch = await response.json();
    if (!Array.isArray(batch))
      fail("GitLab registry returned a non-list response");
    values.push(...batch);
    const next = response.headers.get("x-next-page");
    if (next === "" || (next === null && batch.length < 100)) return values;
    if (next !== null && Number(next) !== page + 1)
      fail("invalid GitLab pagination");
  }
}

async function collect(destination) {
  const api = new URL(process.env.CI_API_V4_URL);
  if (
    !["https:", "http:"].includes(api.protocol) ||
    api.username ||
    api.password
  )
    fail("invalid CI_API_V4_URL");
  const project = `${api.href.replace(/\/$/, "")}/projects/${encodeURIComponent(process.env.CI_PROJECT_ID)}`;
  const packageName = process.env.CI_COMMIT_TAG
    ? "latest"
    : process.env.CI_COMMIT_BRANCH.replaceAll("/", "-");
  const query = new URL(`${project}/packages`);
  query.searchParams.set("package_type", "generic");
  query.searchParams.set("package_name", packageName);
  query.searchParams.set("package_version", version);
  const packages = (await list(query)).filter(
    (item) =>
      item.name === packageName &&
      item.version === version &&
      item.package_type === "generic",
  );
  if (!packages.length) fail(`no GitLab package for ${packageName}/${version}`);
  const files = new Map();
  for (const pkg of packages) {
    if (!Number.isSafeInteger(pkg.id)) fail("invalid GitLab package ID");
    for (const file of await list(
      `${project}/packages/${pkg.id}/package_files`,
    )) {
      const name = file.file_name;
      if (
        typeof name !== "string" ||
        path.basename(name) !== name ||
        !isMatching(name)
      )
        fail("unexpected file in exact-version GitLab package");
      if (isHelper(name)) continue;
      if (!known.has(name) && !isNupkg(name))
        fail(`unknown release deliverable: ${name}`);
      if (
        !Number.isSafeInteger(file.size) ||
        file.size <= 0 ||
        !/^[0-9a-f]{64}$/.test(file.file_sha256)
      )
        fail(`missing size or SHA-256 for ${name}`);
      const previous = files.get(name);
      if (
        previous &&
        (previous.file_sha256 !== file.file_sha256 ||
          previous.size !== file.size)
      )
        fail(`conflicting GitLab package files: ${name}`);
      files.set(name, file);
    }
  }
  for (const [name, file] of [...files].sort(([a], [b]) =>
    a.localeCompare(b),
  )) {
    const url = `${project}/packages/generic/${encodeURIComponent(packageName)}/${encodeURIComponent(version)}/${encodeURIComponent(name)}`;
    const response = await request(url);
    const output = path.join(destination, name);
    await pipeline(
      Readable.fromWeb(response.body),
      createWriteStream(output, { flags: "wx", mode: 0o600 }),
    );
    if (
      (await lstat(output)).size !== file.size ||
      (await digest(output, "sha256")) !== file.file_sha256
    )
      fail(`GitLab download does not match package metadata: ${name}`);
  }
}

async function stage(source, destination) {
  const names = (await readdir(source)).sort();
  let selectedNupkg;
  if (names.includes(sidecar)) {
    const sum = (await readFile(path.join(source, sidecar), "utf8")).trim();
    if (!/^[0-9a-f]{40}$/.test(sum))
      fail("invalid Windows nupkg SHA-1 sidecar");
    selectedNupkg = `${desktopPrefix}windows-amd64.${sum}.nupkg`;
    if (!names.includes(selectedNupkg))
      fail("Windows nupkg named by sidecar is missing");
    if ((await digest(path.join(source, selectedNupkg), "sha1")) !== sum)
      fail("Windows nupkg does not match its SHA-1 sidecar");
  }
  const staged = new Set();
  for (const name of names) {
    if (!isMatching(name) || isHelper(name)) continue;
    if (!known.has(name) && !isNupkg(name))
      fail(`unknown release deliverable: ${name}`);
    // bin/ may contain older hashed nupkgs for the same development version.
    if (isNupkg(name) && selectedNupkg && name !== selectedNupkg) continue;
    // iOS is not a supported Linux producer unless explicitly dispatched.
    if (mode === "build" && name === ios && !macHost) continue;
    const file = path.join(source, name);
    const stat = await lstat(file);
    if (!stat.isFile() || stat.size === 0)
      fail(`release deliverable is not a nonempty regular file: ${name}`);
    await copyFile(file, path.join(destination, name));
    staged.add(name);
  }
  const expected = [...required];
  if (mode === "build" || mode === "ci") expected.push(...apks);
  if (mode === "build" && macHost) expected.push(ios);
  const missing = expected.filter((name) => !staged.has(name));
  if (missing.length)
    fail(`missing required release deliverables:\n  ${missing.join("\n  ")}`);
  if ([...staged].filter(isNupkg).length !== 1)
    fail("expected exactly one current Windows nupkg");
  if (!staged.has(ios))
    console.error(
      "iOS skipped: no macOS producer/archive (the GitLab iOS job is disabled).",
    );
}

let temporary;
try {
  temporary = await mkdtemp(path.join(tmpdir(), "streamplace-release-"));
  let source = path.resolve(directory);
  if (mode === "ci") {
    source = path.join(temporary, "downloads");
    await mkdir(source);
    await collect(source);
  }
  const artifacts = path.join(temporary, "artifacts");
  await mkdir(artifacts);
  await stage(source, artifacts);
  const args = [
    "js/streamplace/scripts/release.mjs",
    "--artifacts",
    artifacts,
    "--version",
    version,
    "--commit",
    commit,
    "--channel",
    channel,
  ];
  if (changelog) args.push("--changelog", changelog);
  if (dryRun === "1") args.push("--dry-run");
  const child = spawn(process.execPath, args, { stdio: "inherit" });
  const code = await new Promise((resolve, reject) => {
    child.on("error", reject);
    child.on("exit", (status, signal) => resolve(signal ? 1 : status));
  });
  process.exitCode = code;
} catch (error) {
  console.error(`release-atproto: ${error.message}`);
  process.exitCode = 1;
} finally {
  if (temporary) await rm(temporary, { recursive: true, force: true });
}
