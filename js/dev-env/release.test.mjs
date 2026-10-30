import assert from "node:assert/strict";
import { execFile } from "node:child_process";
import { createHash, randomUUID } from "node:crypto";
import { mkdir, mkdtemp, readFile, rm, writeFile } from "node:fs/promises";
import { tmpdir } from "node:os";
import { join } from "node:path";
import test from "node:test";
import { fileURLToPath } from "node:url";
import { promisify } from "node:util";
import { TestNetwork } from "./dist/index.js";

const exec = promisify(execFile);
const repo = fileURLToPath(new URL("../../", import.meta.url));
const publisher = join(repo, "hack/release-atproto.mjs");

test(
  "release publication preserves native artifacts and immutable history",
  { timeout: 120_000 },
  async (t) => {
    const directory = await mkdtemp(join(tmpdir(), "streamplace-release-"));
    t.after(() => rm(directory, { recursive: true, force: true }));
    const network = await TestNetwork.create({});
    t.after(() => network.close());
    const password = randomUUID();
    const created = await fetch(
      `${network.pds.url}/xrpc/com.atproto.server.createAccount`,
      {
        method: "POST",
        headers: { "content-type": "application/json" },
        body: JSON.stringify({
          handle: "release-test.test",
          email: "release-test@example.com",
          password,
        }),
      },
    );
    assert.equal(created.status, 200, await created.clone().text());
    const { did } = await created.json();
    const { stdout } = await exec("git", ["rev-parse", "HEAD"], { cwd: repo });
    const commit = stdout.trim();
    const env = {
      ...process.env,
      SP_RELEASE_PDS: network.pds.url,
      SP_RELEASE_DID: did,
      SP_RELEASE_IDENTIFIER: did,
      SP_RELEASE_PASSWORD: password,
    };
    async function publish(version, artifacts, overrides = {}) {
      return exec(
        process.execPath,
        [
          publisher,
          "publish",
          artifacts,
          version,
          commit,
          "latest",
          "",
          "0",
          "",
        ],
        { cwd: repo, env: { ...env, ...overrides } },
      );
    }
    async function records(collection) {
      const url = new URL(
        "/xrpc/com.atproto.repo.listRecords",
        network.pds.url,
      );
      url.search = new URLSearchParams({ repo: did, collection, limit: "100" });
      const response = await fetch(url);
      assert.equal(response.status, 200);
      return (await response.json()).records;
    }
    async function fixture(version) {
      const path = join(directory, version);
      await mkdir(path);
      const files = new Map();
      for (const arch of ["amd64", "arm64"]) {
        for (const suffix of [
          `linux-${arch}.tar.gz`,
          `linux-${arch}.deb`,
          `darwin-${arch}.tar.gz`,
        ]) {
          files.set(`streamplace-${version}-${suffix}`, Buffer.from(suffix));
        }
        for (const suffix of [
          `linux-${arch}.AppImage`,
          `darwin-${arch}.zip`,
          `darwin-${arch}.dmg`,
        ]) {
          files.set(
            `streamplace-desktop-${version}-${suffix}`,
            Buffer.from(suffix),
          );
        }
      }
      files.set(
        `streamplace-${version}-windows-amd64.zip`,
        Buffer.from("windows node"),
      );
      files.set(
        `streamplace-${version}-ios-release.xcarchive.tar.gz`,
        Buffer.from("ios archive"),
      );
      files.set(
        `streamplace-desktop-${version}-windows-amd64.exe`,
        Buffer.from("windows desktop"),
      );
      for (const flavor of ["debug", "release"]) {
        for (const ext of ["apk", "aab"]) {
          files.set(
            `streamplace-${version}-android-${flavor}.${ext}`,
            Buffer.from(flavor + ext),
          );
        }
      }
      const nupkg = Buffer.from("windows update");
      const sha1 = createHash("sha1").update(nupkg).digest("hex");
      files.set(
        `streamplace-desktop-${version}-windows-amd64.${sha1}.nupkg`,
        nupkg,
      );
      for (const helper of [
        `streamplace-${version}-android-e2e.apk`,
        `streamplace-${version}-android-e2e.aab`,
        `streamplace-${version}-android-e2e.apks`,
        `streamplace-${version}-android-debug.apks`,
        `streamplace-${version}-android-release.apks`,
        `streamplace-${version}-SHA256SUMS`,
        `streamplace-desktop-${version}-windows-amd64.nupkg`,
        `streamplace-${version}-12345678-linux-amd64.tar.gz`,
      ]) {
        await writeFile(join(path, helper), "must not publish");
      }
      await writeFile(
        join(path, `streamplace-desktop-${version}-windows-amd64.nupkg.sha1`),
        sha1,
      );
      await mkdir(join(path, `streamplace-${version}-ios-release.xcarchive`));
      for (const [name, bytes] of files)
        await writeFile(join(path, name), bytes);
      return { path, files };
    }

    const version = `v0.0.0-release-${randomUUID()}`;
    const first = await fixture(version);
    const concurrent = await Promise.allSettled([
      publish(version, first.path),
      publish(version, first.path),
    ]);
    assert.ok(
      concurrent.some((result) => result.status === "fulfilled"),
      concurrent.map((result) => result.reason?.message).join("\n"),
    );
    const projects = await records("software.release.project");
    assert.equal(projects.length, 2);
    const desktop = projects.find(
      (record) => record.value.name === "streamplace-desktop",
    );
    assert.equal(
      desktop.value.parent,
      `at://${did}/software.release.project/streamplace`,
    );
    const manifests = await records("software.release.manifest");
    const versions = await records("software.release.version");
    const channels = await records("software.release.channel");
    assert.equal(manifests.length, 2);
    assert.equal(versions.length, 2);
    assert.equal(channels.length, 2);
    const seen = new Set();
    for (const record of versions) {
      const project = record.value.project.split("/").at(-1);
      assert.equal(
        record.uri,
        `at://${did}/software.release.version/${project}.${version}`,
      );
      const manifest = manifests.find(
        (item) => item.uri === record.value.manifest.uri,
      );
      assert.equal(record.value.manifest.cid, manifest.cid);
      assert.equal(manifest.value.project, record.value.project);
      const source = manifest.value.artifacts.filter(
        (item) => item.$type === "software.release.defs#gitRevision",
      );
      assert.deepEqual(source, [
        {
          $type: "software.release.defs#gitRevision",
          repository: "https://git.stream.place/streamplace/streamplace.git",
          commit: { algorithm: "sha1", digest: commit },
        },
      ]);
      const channel = channels.find(
        (item) => item.value.project === record.value.project,
      );
      assert.equal(
        channel.uri,
        `at://${did}/software.release.channel/${project}.latest`,
      );
      assert.equal(channel.value.channel, "latest");
      assert.deepEqual(channel.value.version, {
        uri: record.uri,
        cid: record.cid,
      });
      for (const artifact of manifest.value.artifacts.filter(
        (item) => item.$type === "software.release.defs#blobArtifact",
      )) {
        assert.equal(
          artifact.fileName.startsWith("streamplace-desktop-"),
          project === "streamplace-desktop",
        );
        const expected = first.files.get(artifact.fileName);
        assert.ok(expected, artifact.fileName);
        assert.equal(artifact.blob.size, expected.length);
        const os = artifact.fileName.includes("-darwin-")
          ? "macos"
          : artifact.fileName.includes("-windows-")
            ? "windows"
            : artifact.fileName.includes("-ios-")
              ? "ios"
              : artifact.fileName.includes("-linux-")
                ? "linux"
                : "android";
        assert.equal(artifact.target.os, os);
        assert.deepEqual(
          artifact.target.architectures,
          os === "android" || os === "ios"
            ? undefined
            : [artifact.fileName.includes("-arm64") ? "arm64" : "amd64"],
        );
        const url = new URL("/xrpc/com.atproto.sync.getBlob", network.pds.url);
        url.search = new URLSearchParams({ did, cid: artifact.blob.ref.$link });
        const downloaded = await fetch(url);
        assert.equal(downloaded.status, 200);
        assert.deepEqual(Buffer.from(await downloaded.arrayBuffer()), expected);
        seen.add(artifact.fileName);
      }
    }
    assert.deepEqual(seen, new Set(first.files.keys()));

    const secondVersion = `v0.0.1-release-${randomUUID()}`;
    const second = await fixture(secondVersion);
    await publish(secondVersion, second.path);
    const newerChannels = await records("software.release.channel");
    assert.ok(
      newerChannels.every((item) =>
        item.value.version.uri.endsWith(`.${secondVersion}`),
      ),
    );
    await publish(version, first.path);
    assert.deepEqual(await records("software.release.channel"), newerChannels);
    assert.equal((await records("software.release.manifest")).length, 4);
    assert.equal((await records("software.release.version")).length, 4);

    const modified = join(first.path, first.files.keys().next().value);
    await writeFile(
      modified,
      Buffer.concat([await readFile(modified), Buffer.from("changed")]),
    );
    await assert.rejects(publish(version, first.path));
    assert.deepEqual(await records("software.release.channel"), newerChannels);
    assert.equal((await records("software.release.manifest")).length, 4);
    assert.deepEqual(
      (await records("software.release.version")).filter(
        (item) => item.value.version === version,
      ),
      versions,
    );

    await assert.rejects(
      publish(secondVersion, second.path, {
        SP_RELEASE_DID: "did:plc:aaaaaaaaaaaaaaaaaaaaaaaa",
      }),
    );
    assert.equal((await records("software.release.project")).length, 2);
  },
);
