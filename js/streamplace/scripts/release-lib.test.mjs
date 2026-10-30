import { CID } from "multiformats/cid";
import * as Digest from "multiformats/hashes/digest";
import assert from "node:assert/strict";
import { createHash } from "node:crypto";
import { mkdir, mkdtemp, rm, writeFile } from "node:fs/promises";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { after, before, describe, it } from "node:test";
import {
  PROJECTS,
  buildInventory,
  localBlobRef,
  planRelease,
  validateInputs,
  validateRecord,
} from "./release-lib.mjs";

const DID = "did:plc:rbvrr34edl5ddpuwcubjiost";
const VERSION = "v1.2.3";
const COMMIT = "29e8e011091d3a5c1d2aac73a42f85786e0b16e2";

const cidOf = (text) =>
  CID.createV1(
    0x55,
    Digest.create(0x12, createHash("sha256").update(text).digest()),
  );

const NODE = [
  "linux-amd64.tar.gz",
  "linux-amd64.deb",
  "linux-arm64.tar.gz",
  "linux-arm64.deb",
  "darwin-amd64.tar.gz",
  "darwin-arm64.tar.gz",
  "windows-amd64.zip",
  "android-debug.apk",
  "android-debug.aab",
  "android-release.apk",
  "android-release.aab",
];
const DESKTOP = [
  "darwin-amd64.dmg",
  "darwin-amd64.zip",
  "darwin-arm64.dmg",
  "darwin-arm64.zip",
  "linux-amd64.AppImage",
  "linux-arm64.AppImage",
  "windows-amd64.exe",
  `windows-amd64.${"a".repeat(40)}.nupkg`,
];

async function release(dir, version = VERSION) {
  await mkdir(dir, { recursive: true });
  const names = [
    ...NODE.map((n) => `streamplace-${version}-${n}`),
    ...DESKTOP.map((n) => `streamplace-desktop-${version}-${n}`),
  ];
  for (const name of names) await writeFile(join(dir, name), name);
  return names;
}

describe("inventory", () => {
  let root;
  before(async () => {
    root = await mkdtemp(join(tmpdir(), "release-lib-"));
  });
  after(() => rm(root, { recursive: true, force: true }));

  it("groups a shared bin directory by project and ignores other builds", async () => {
    const dir = join(root, "bin");
    const names = await release(dir);
    // Build helpers and other versions' files.
    for (const extra of [
      `streamplace-desktop-${VERSION}-windows-amd64.nupkg`,
      `streamplace-desktop-${VERSION}-windows-amd64.nupkg.sha1`,
      `streamplace-${VERSION}-android-e2e.apks`,
      `streamplace-${VERSION}-android-e2e.apk`,
      `streamplace-${VERSION}-android-debug.apks`,
      `streamplace-${VERSION}-android-release.apks`,
      `streamplace-${VERSION}-5269a091-android-release.apk`,
      "streamplace-v1.2.30-linux-amd64.deb",
      "streamplace-v1.2.2-linux-amd64.deb",
      "notes.txt",
    ]) {
      await writeFile(join(dir, extra), "x");
    }
    await mkdir(join(dir, `streamplace-${VERSION}-ios-release.xcarchive`));
    await writeFile(join(dir, `streamplace-${VERSION}-SHA256SUMS`), "sums");

    const projects = await buildInventory({ dir, version: VERSION });
    const all = projects.flatMap((p) => p.files);
    assert.deepEqual(new Set(all.map((file) => file.fileName)), new Set(names));
    const byName = (n) => all.find((f) => f.fileName.endsWith(n));
    assert.deepEqual(byName("darwin-arm64.tar.gz").target, {
      os: "macos",
      architectures: ["arm64"],
    });
    // Android flavor is not an architecture.
    assert.deepEqual(byName("android-release.apk").target, { os: "android" });
  });

  it("does not take a dev build's files for the stable version, or vice versa", async () => {
    const dir = join(root, "dev");
    await release(dir, "v1.2.3-5269a091");
    await writeFile(join(dir, "streamplace-v1.2.3-linux-amd64.tar.gz"), "x");
    const projects = await buildInventory({ dir, version: "v1.2.3-5269a091" });
    assert.ok(
      projects
        .flatMap((p) => p.files)
        .every((f) => f.fileName.includes("v1.2.3-5269a091-")),
    );
    const stable = await buildInventory({ dir, version: VERSION });
    assert.deepEqual(
      stable.flatMap((p) => p.files.map((f) => f.fileName)),
      ["streamplace-v1.2.3-linux-amd64.tar.gz"],
    );
  });

  it("fails on unknown version-matching files instead of omitting them", async () => {
    const dir = join(root, "unknown");
    await release(dir);
    await writeFile(
      join(dir, `streamplace-${VERSION}-linux-riscv64.tar.gz`),
      "x",
    );
    await assert.rejects(
      buildInventory({ dir, version: VERSION }),
      /unrecognized files[^]*riscv64/,
    );
  });

  it("publishes whatever is present and omits projects with nothing", async () => {
    const dir = join(root, "partial");
    await mkdir(dir);
    const name = `streamplace-${VERSION}-darwin-arm64.tar.gz`;
    await writeFile(join(dir, name), "x");
    const projects = await buildInventory({ dir, version: VERSION });
    assert.deepEqual(
      projects.map((p) => [p.name, p.files.map((f) => f.fileName)]),
      [["streamplace", [name]]],
    );
    await assert.rejects(
      buildInventory({ dir, version: "v9.9.9" }),
      /no v9\.9\.9 deliverables/,
    );
  });
});

describe("validation", () => {
  it("rejects malformed inputs", () => {
    const ok = { version: VERSION, channel: "latest", commit: COMMIT };
    assert.throws(() => validateInputs({ ...ok, version: "1.2.3" }), /version/);
    assert.throws(() => validateInputs({ ...ok, channel: "a.b" }), /channel/);
    assert.throws(() => validateInputs({ ...ok, commit: "29e8e01" }), /commit/);
  });

  it("checks artifact fragments inside the open manifest union", () => {
    const manifest = (fileName) => ({
      $type: "software.release.manifest",
      project: `at://${DID}/software.release.project/streamplace`,
      artifacts: [
        {
          $type: "software.release.defs#blobArtifact",
          fileName,
          blob: localBlobRef({
            cid: "bafkreihdo2ia4mlfggapo4l4tvwzjpil4g6siqsxv7ogqyadmivwhd25j4",
            mimeType: "text/plain",
            size: 1,
          }),
        },
      ],
    });
    validateRecord(manifest("fine.txt"));
    assert.throws(() => validateRecord(manifest("../evil")), /unsafe/);
    assert.throws(() => validateRecord(manifest("..")), /unsafe/);
    const missingBlob = manifest("x");
    delete missingBlob.artifacts[0].blob;
    assert.throws(() => validateRecord(missingBlob));
  });
});

describe("planRelease", () => {
  const source = {
    $type: "software.release.defs#gitRevision",
    repository: "https://git.stream.place/streamplace/streamplace.git",
    commit: { algorithm: "sha1", digest: COMMIT },
  };
  const file = (project, fileName, n) => ({
    project,
    fileName,
    path: fileName,
    size: n,
    cid: cidOf(fileName),
    mimeType: "application/octet-stream",
    target: { os: "linux", architectures: ["amd64"] },
  });
  const projects = PROJECTS.map((name, i) => ({
    name,
    files: [file(name, `${name}-${VERSION}-linux-amd64.tar.gz`, i + 1)],
  }));
  const empty = () => ({
    projects: {},
    versions: {},
    channels: {},
    schemas: {},
  });
  const run = (existing, extra = {}, files = projects) =>
    planRelease({
      did: DID,
      version: VERSION,
      channel: "latest",
      source,
      projects: files,
      blobs: new Map(
        files.flatMap((p) => p.files).map((f) => [f.fileName, localBlobRef(f)]),
      ),
      schemaDocs: [],
      existing,
      ...extra,
    });

  // What the repo holds after a plan was written.
  const apply = ({ writes }) => {
    const existing = empty();
    for (const w of writes) {
      const name = w.value.name ?? w.value.project?.split("/").at(-1);
      if (w.collection === "software.release.project") {
        existing.projects[w.rkey] = w.value;
      } else if (w.collection === "software.release.channel") {
        existing.channels[name] = w.value;
      } else if (w.collection === "software.release.version") {
        const manifest = writes.find((m) => m.uri === w.value.manifest.uri);
        existing.versions[name] = {
          record: w.value,
          cid: w.cid,
          manifest: { record: manifest.value, cid: manifest.cid },
        };
      }
    }
    return existing;
  };

  it("fails when a published version's contents differ", () => {
    const existing = apply(run(empty()));
    const changed = projects.map((p) => ({
      ...p,
      files: p.files.map((f) => ({ ...f, cid: cidOf("changed") })),
    }));
    assert.throws(() => run(existing, {}, changed), /immutable[^]*differs/);
    assert.throws(
      () => run(existing, { changelog: "https://example.com/changelog" }),
      /changelog/,
    );
    const otherCommit = {
      ...source,
      commit: { algorithm: "sha1", digest: "b".repeat(40) },
    };
    assert.throws(() => run(existing, { source: otherCommit }), /immutable/);
  });

  it("moves an existing channel to a new version and recreates a missing one", () => {
    const published = apply(run(empty()));
    const next = planRelease({
      did: DID,
      version: "v1.2.4",
      channel: "latest",
      source,
      projects,
      blobs: new Map(
        projects
          .flatMap((p) => p.files)
          .map((f) => [f.fileName, localBlobRef(f)]),
      ),
      schemaDocs: [],
      existing: { ...published, versions: {} },
    });
    assert.deepEqual(
      next.writes.filter((w) => w.action === "update").map((w) => w.rkey),
      PROJECTS.map((p) => `${p}.latest`),
    );

    const noChannel = { ...published, channels: {} };
    const restored = run(noChannel);
    assert.deepEqual(
      restored.writes.map((w) => `${w.action} ${w.rkey}`),
      PROJECTS.map((p) => `create ${p}.latest`),
    );
    assert.equal(
      restored.writes[0].value.version.cid,
      published.versions.streamplace.cid,
    );
  });

  it("refuses a project record that does not match", () => {
    const existing = apply(run(empty()));
    existing.projects["streamplace-desktop"] = {
      ...existing.projects["streamplace-desktop"],
      parent: undefined,
    };
    assert.throws(() => run(existing), /project record/);
  });
});
