// Offline half of the ATProto release publisher: artifact inventory, native
// record construction, local schema validation, and the publish plan. Nothing
// in here talks to a PDS; release.mjs does. See release.mjs for the CLI.
import { schemas } from "@atproto/api";
import { jsonToLex as nativeJsonToLex } from "@atproto/lex";
import { encode } from "@atproto/lex-cbor";
import { Lexicons, jsonToLex } from "@atproto/lexicon";
import { CID } from "multiformats/cid";
import * as Digest from "multiformats/hashes/digest";
import { createHash } from "node:crypto";
import { createReadStream, readFileSync } from "node:fs";
import { lstat, readdir } from "node:fs/promises";
import { join } from "node:path";
import { isDeepStrictEqual as sameValue } from "node:util";

export const DEFAULT_DID = "did:plc:rbvrr34edl5ddpuwcubjiost";
export const DEFAULT_REPOSITORY =
  "https://git.stream.place/streamplace/streamplace.git";

const NS = "software.release";
export const PROJECT = `${NS}.project`;
export const MANIFEST = `${NS}.manifest`;
export const VERSION = `${NS}.version`;
export const CHANNEL = `${NS}.channel`;
const BLOB_ARTIFACT = `${NS}.defs#blobArtifact`;
const GIT_REVISION = `${NS}.defs#gitRevision`;

// `streamplace-desktop` is a child of `streamplace`. A project's files are named
// `<project>-<version>-<rest>`; `rest` is matched against ARTIFACTS below.
const PARENT_PROJECT = "streamplace";
export const PROJECTS = [PARENT_PROJECT, "streamplace-desktop"];

const ARCHES = ["amd64", "arm64"];
const GZIP = "application/gzip";
const ZIP = "application/zip";
const BINARY = "application/octet-stream";

// A deliverable: the filename after `<project>-<version>-`, the target it runs
// on, and the MIME type we declare to the PDS (which may normalize it). Target
// architecture is only listed where the artifact is actually per-architecture:
// an Android/iOS build is neither, and its debug/release flavor is not one.
const artifact = (name, os, arch, mimeType) => ({ name, os, arch, mimeType });
const ARTIFACTS = {
  streamplace: [
    ...ARCHES.flatMap((arch) => [
      artifact(`linux-${arch}.tar.gz`, "linux", arch, GZIP),
      artifact(
        `linux-${arch}.deb`,
        "linux",
        arch,
        "application/vnd.debian.binary-package",
      ),
      artifact(`darwin-${arch}.tar.gz`, "macos", arch, GZIP),
    ]),
    artifact("windows-amd64.zip", "windows", "amd64", ZIP),
    ...["debug", "release"].flatMap((flavor) => [
      artifact(
        `android-${flavor}.apk`,
        "android",
        undefined,
        "application/vnd.android.package-archive",
      ),
      artifact(`android-${flavor}.aab`, "android", undefined, BINARY),
    ]),
    artifact("ios-release.xcarchive.tar.gz", "ios", undefined, GZIP),
  ],
  "streamplace-desktop": [
    ...ARCHES.flatMap((arch) => [
      artifact(
        `darwin-${arch}.dmg`,
        "macos",
        arch,
        "application/x-apple-diskimage",
      ),
      artifact(`darwin-${arch}.zip`, "macos", arch, ZIP),
      artifact(`linux-${arch}.AppImage`, "linux", arch, BINARY),
    ]),
    artifact(
      "windows-amd64.exe",
      "windows",
      "amd64",
      "application/vnd.microsoft.portable-executable",
    ),
    // The Squirrel package name embeds its own SHA-1.
    artifact(/^windows-amd64\.[0-9a-f]{40}\.nupkg$/, "windows", "amd64", ZIP),
  ],
};

// Build intermediates and sidecars, never release downloads. The signing
// workflow's SHA256SUMS covers only macOS; native blob CIDs cover every download.
const BUILD_HELPERS = [
  /^windows-amd64\.nupkg(\.sha1)?$/,
  /^android-e2e\.(apk|aab|apks)$/,
  /^android-(debug|release)\.apks$/,
  /^sign-input(\.tar\.gz)?$/,
  /^ios-release\.xcarchive$/,
  /^SHA256SUMS$/,
];

// Stable `vX.Y.Z` or a dev/pre-release `vX.Y.Z-<suffix>` (`make version` emits
// the commit hash prefix), within the lexicon's version grammar.
const VERSION_RE = /^v\d+\.\d+\.\d+(-[a-z0-9]+)*$/;
// The project-name grammar from the lexicon prose: no dots.
const CHANNEL_RE = /^[a-z0-9]+(-[a-z0-9]+)*$/;
const COMMIT_RE = /^([0-9a-f]{40}|[0-9a-f]{64})$/;

export function validateInputs({ version, channel, commit }) {
  if (!VERSION_RE.test(version) || version.length > 64) {
    throw new Error(
      `version ${JSON.stringify(version)} must look like v1.2.3 or v1.2.3-<suffix>`,
    );
  }
  if (!CHANNEL_RE.test(channel) || channel.length > 64) {
    throw new Error(
      `channel ${JSON.stringify(channel)} must be lowercase alphanumerics and interior hyphens (max 64)`,
    );
  }
  if (!COMMIT_RE.test(commit)) {
    throw new Error("commit must be a full lowercase 40- or 64-digit hex hash");
  }
}

// Shared by staging and publication so a stable release cannot collect a
// prerelease's files merely because its version is a prefix of theirs.
export function releaseProjectForFile(fileName, version) {
  for (const project of PROJECTS) {
    const prefix = `${project}-${version}-`;
    if (!fileName.startsWith(prefix)) continue;
    const rest = fileName.slice(prefix.length);
    if (
      /^[a-z0-9]+(?:-[a-z0-9]+)*-(linux|darwin|windows|android|ios)-/.test(rest)
    ) {
      return undefined;
    }
    return project;
  }
}

export function isReleaseBuildHelper(fileName, version) {
  const project = releaseProjectForFile(fileName, version);
  if (!project) return false;
  const rest = fileName.slice(`${project}-${version}-`.length);
  return BUILD_HELPERS.some((pattern) => pattern.test(rest));
}

// --- inventory --------------------------------------------------------------

/**
 * Finds this version's deliverables in `dir`, hashes them, and groups them by
 * project. A file named for this version that is neither a deliverable nor a
 * known build helper is an error; other versions' files in a shared bin
 * directory are ignored. Which deliverables a release should have is up to the
 * caller: whatever is present is published, and a published version is
 * immutable, so it can never be completed later.
 */
export async function buildInventory({ dir, version }) {
  const names = await readdir(dir);
  const files = [];
  const unknown = [];
  for (const entry of names.sort()) {
    const project = releaseProjectForFile(entry, version);
    if (!project) continue;
    if (isReleaseBuildHelper(entry, version)) continue;
    const path = join(dir, entry);
    const rest = entry.slice(`${project}-${version}-`.length);
    const entryStat = await lstat(path);
    if (!entryStat.isFile()) {
      throw new Error(`release artifact must be a regular file: ${entry}`);
    }
    const rule = ARTIFACTS[project].find((r) =>
      typeof r.name === "string" ? r.name === rest : r.name.test(rest),
    );
    if (rule) files.push({ project, entry, path, rule });
    else unknown.push(entry);
  }
  if (unknown.length > 0) {
    throw new Error(
      `unrecognized files for ${version} in ${dir} (not a known deliverable or build helper):\n  ${unknown.join("\n  ")}`,
    );
  }

  if (files.length === 0) {
    throw new Error(`no ${version} deliverables found in ${dir}`);
  }

  const hashed = [];
  for (const { project, entry, path, rule } of files) {
    const { size, cid } = await hashFile(path);
    if (size === 0) throw new Error(`${entry} is empty`);
    hashed.push({
      project,
      fileName: entry,
      path,
      size,
      cid,
      mimeType: rule.mimeType,
      target: rule.os
        ? {
            os: rule.os,
            ...(rule.arch && { architectures: [rule.arch] }),
          }
        : undefined,
    });
  }
  // A project with nothing to publish is left out.
  return PROJECTS.map((name) => ({
    name,
    files: hashed.filter((f) => f.project === name),
  })).filter((p) => p.files.length > 0);
}

const RAW = 0x55;
const DAG_CBOR = 0x71;
const SHA2_256 = 0x12;

/** Streams a file once, returning its size and blob CID (CIDv1, raw, sha-256). */
async function hashFile(path) {
  const hash = createHash("sha256");
  let size = 0;
  for await (const chunk of createReadStream(path)) {
    hash.update(chunk);
    size += chunk.length;
  }
  return {
    size,
    cid: CID.createV1(RAW, Digest.create(SHA2_256, hash.digest())),
  };
}

/** The blob ref as it appears in a record, for a blob the PDS has not seen yet. */
export const localBlobRef = (file) => ({
  $type: "blob",
  ref: { $link: file.cid.toString() },
  mimeType: file.mimeType,
  size: file.size,
});

// --- records ----------------------------------------------------------------

const atUri = (did, collection, rkey) => `at://${did}/${collection}/${rkey}`;

export function gitRevision({ repository, commit, ref }) {
  return {
    $type: GIT_REVISION,
    repository,
    commit: {
      algorithm: commit.length === 64 ? "sha256" : "sha1",
      digest: commit,
    },
    ...(ref && { ref }),
  };
}

function blobArtifact(file, blob) {
  return {
    $type: BLOB_ARTIFACT,
    fileName: file.fileName,
    blob,
    ...(file.target && { target: file.target }),
  };
}

const projectRecord = (did, name) => ({
  $type: PROJECT,
  name,
  ...(name !== PARENT_PROJECT && {
    parent: atUri(did, PROJECT, PARENT_PROJECT),
  }),
});

const manifestRecord = (did, name, artifacts) => ({
  $type: MANIFEST,
  project: atUri(did, PROJECT, name),
  artifacts,
});

const versionRecord = (did, name, version, manifest, changelog) => ({
  $type: VERSION,
  project: atUri(did, PROJECT, name),
  version,
  manifest,
  ...(changelog && { changelog }),
});

const channelRecord = (did, name, channel, versionRef) => ({
  $type: CHANNEL,
  project: atUri(did, PROJECT, name),
  channel,
  version: versionRef,
});

// Strong references in one atomic write require canonical record CIDs up front.
// Reuse the ATProto encoder rather than maintaining a second DAG-CBOR codec.

/** The CID the PDS assigns to a record: CIDv1, dag-cbor, sha-256. */
const recordCid = (record) =>
  CID.createV1(
    DAG_CBOR,
    Digest.create(
      SHA2_256,
      createHash("sha256")
        .update(encode(nativeJsonToLex(record)))
        .digest(),
    ),
  );

const B32 = "234567abcdefghijklmnopqrstuvwxyz";
let lastMicros = 0;

/** A new record key of type `tid`: 53 bits of microseconds, 10 of clock id. */
function newTid() {
  lastMicros = Math.max(Date.now() * 1000, lastMicros + 1);
  let n =
    (BigInt(lastMicros) << 10n) | BigInt(Math.floor(Math.random() * 1024));
  let tid = "";
  for (let i = 0; i < 13; i++, n >>= 5n) tid = B32[Number(n & 31n)] + tid;
  return tid;
}

// --- validation -------------------------------------------------------------

let lexicons;
/** The pinned release lexicons, plus the strongRef they reference. */
function releaseLexicons() {
  if (!lexicons) {
    const { lexicons: docs } = JSON.parse(
      readFileSync(new URL("./release-schemas.json", import.meta.url), "utf8"),
    );
    lexicons = new Lexicons(docs);
    lexicons.add(
      schemas.find((doc) => doc.id === "com.atproto.repo.strongRef"),
    );
  }
  return lexicons;
}

const UNSAFE_FILENAME = /[/\\\u0000-\u001f\u007f]/;

/**
 * Validates a record against the pinned schemas. The manifest's `artifacts` is
 * an open union, which lexicon validation accepts without looking inside, so
 * each fragment is validated against its own definition; the schema prose
 * rules the lexicon cannot express are checked here.
 */
export function validateRecord(record) {
  const lex = releaseLexicons();
  lex.assertValidRecord(record.$type, jsonToLex(record));
  for (const fragment of record.artifacts ?? []) {
    const result = lex.validate(fragment.$type, jsonToLex(fragment));
    if (!result.success) throw result.error;
    if (fragment.$type === BLOB_ARTIFACT) {
      const { fileName } = fragment;
      if (UNSAFE_FILENAME.test(fileName) || /^\.\.?$/.test(fileName)) {
        throw new Error(`unsafe artifact fileName ${JSON.stringify(fileName)}`);
      }
    } else if (fragment.$type === GIT_REVISION) {
      const { algorithm, digest } = fragment.commit;
      if (digest.length !== (algorithm === "sha256" ? 64 : 40)) {
        throw new Error(`commit digest does not match algorithm ${algorithm}`);
      }
    }
  }
}

// --- plan -------------------------------------------------------------------

const artifactKey = (a) =>
  a.$type === BLOB_ARTIFACT ? `${a.fileName}` : a.$type;
// The PDS may normalize MIME types; a Git ref is descriptive context only.
// Preserve all other source and artifact fields when comparing immutable data.
function identity(a) {
  if (a.$type === BLOB_ARTIFACT) {
    const { mimeType, ...blob } = a.blob;
    return { ...a, blob };
  }
  const { ref, ...revision } = a;
  return revision;
}

function manifestDiff(have, want) {
  const problems = [];
  if (have.project !== want.project) {
    problems.push(`project is ${have.project}, not ${want.project}`);
  }
  const index = (m) => new Map(m.artifacts.map((a) => [artifactKey(a), a]));
  const haveIndex = index(have);
  const wantIndex = index(want);
  for (const [key, w] of wantIndex) {
    const h = haveIndex.get(key);
    if (!h) problems.push(`${key} is not in the published manifest`);
    else if (!sameValue(identity(h), identity(w))) {
      problems.push(`${key} differs from the published one`);
    }
  }
  for (const key of haveIndex.keys()) {
    if (!wantIndex.has(key))
      problems.push(`${key} is only in the published manifest`);
  }
  return problems;
}

/**
 * Decides what to write. `existing` is what the repo holds now:
 *   projects[name]  -> record | undefined
 *   versions[name]  -> {record, cid, manifest: {record, cid}} | undefined
 *   channels[name]  -> record | undefined
 * `blobs` maps fileName to the blob ref to publish. Published versions and their
 * manifests are immutable: identical inputs are left alone, anything else
 * throws. A channel only moves when this run publishes a new version, so
 * re-running an old release never points a channel back at it.
 *
 * Returns {writes, created, notes}; `created` names the projects whose new
 * version needs its blobs uploaded.
 */
export function planRelease({
  did,
  version,
  channel,
  changelog,
  source,
  projects,
  blobs,
  existing,
}) {
  const writes = [];
  const created = [];
  const notes = [];
  const write = (action, collection, rkey, value) => {
    validateRecord(value);
    writes.push({
      action,
      collection,
      rkey,
      uri: atUri(did, collection, rkey),
      cid: recordCid(value).toString(),
      value,
    });
    return writes.at(-1);
  };

  for (const name of PROJECTS) {
    const want = projectRecord(did, name);
    const have = existing.projects[name];
    if (!have) write("create", PROJECT, name, want);
    else if (
      have.name !== name ||
      have.parent !== want.parent ||
      have.renamedTo
    ) {
      throw new Error(
        `project record ${name} in the repo does not match: ${JSON.stringify(have)}`,
      );
    }
  }

  for (const { name, files } of projects) {
    const versionKey = `${name}.${version}`;
    const channelKey = `${name}.${channel}`;
    const projectUri = atUri(did, PROJECT, name);
    const have = existing.versions[name];
    let versionRef;
    if (have) {
      const manifest = manifestRecord(did, name, [
        source,
        ...files.map((f) => blobArtifact(f, blobs.get(f.fileName))),
      ]);
      const problems = manifestDiff(have.manifest.record, manifest);
      if (have.record.project !== projectUri) {
        problems.push(`version record belongs to ${have.record.project}`);
      }
      if (changelog && have.record.changelog !== changelog) {
        problems.push(
          `changelog is ${have.record.changelog}, not ${changelog}`,
        );
      }
      if (problems.length > 0) {
        throw new Error(
          `${versionKey} is already published and releases are immutable, but this run differs:\n  ${problems.join("\n  ")}`,
        );
      }
      versionRef = {
        uri: atUri(did, VERSION, versionKey),
        cid: have.cid,
      };
      notes.push(`${versionKey} is already published with identical contents`);
    } else {
      const tid = newTid();
      const manifest = write(
        "create",
        MANIFEST,
        tid,
        manifestRecord(did, name, [
          source,
          ...files.map((f) => {
            const blob = blobs.get(f.fileName);
            if (!blob) throw new Error(`${f.fileName} has not been uploaded`);
            return blobArtifact(f, blob);
          }),
        ]),
      );
      const record = write(
        "create",
        VERSION,
        versionKey,
        versionRecord(
          did,
          name,
          version,
          { uri: manifest.uri, cid: manifest.cid },
          changelog,
        ),
      );
      versionRef = { uri: record.uri, cid: record.cid };
      created.push(name);
    }

    const channelHave = existing.channels[name];
    if (channelHave) {
      if (
        channelHave.project !== projectUri ||
        channelHave.channel !== channel
      ) {
        throw new Error(
          `channel record ${channelKey} in the repo does not match: ${JSON.stringify(channelHave)}`,
        );
      }
    }
    if (!channelHave) {
      write(
        "create",
        CHANNEL,
        channelKey,
        channelRecord(did, name, channel, versionRef),
      );
    } else if (have) {
      const note =
        channelHave.version?.uri === versionRef.uri
          ? "already points here"
          : "points elsewhere, left unchanged";
      notes.push(`${channelKey} ${note}`);
    } else {
      write(
        "update",
        CHANNEL,
        channelKey,
        channelRecord(did, name, channel, versionRef),
      );
    }
  }
  return { writes, created, notes };
}
