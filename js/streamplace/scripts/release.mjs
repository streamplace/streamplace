#!/usr/bin/env node
// Publishes a Streamplace release to the ATProto repo of the publishing account
// as native software.release.* records, with every artifact stored as a blob in
// that account's PDS.
//
//   node js/streamplace/scripts/release.mjs \
//     --artifacts DIR --version v1.2.3 --commit FULL_COMMIT --channel latest \
//     [--changelog URL] [--dry-run]
//
// DIR holds the version's deliverables (usually bin/). Two projects are
// published: `streamplace` and its child `streamplace-desktop`; each gets an
// immutable manifest, an immutable version record `<project>.<version>`, and
// moves the mutable channel record `<project>.<channel>`. The checkout's
// place.stream.* lexicons (lexicons/place/stream, including the generated
// authFull permission set) are published as com.atproto.lexicon.schema records
// keyed by NSID. Everything lands in a single atomic applyWrites, after the
// blobs are uploaded.
//
// --dry-run is offline: it hashes the files and prints the records that would
// be written, assuming an empty repo, as JSON on stdout. Nothing is read from
// or sent to a PDS.
//
// Environment (credentials never go on the command line):
//   SP_RELEASE_PDS         PDS URL; required unless --dry-run
//   SP_RELEASE_PASSWORD    app password; required unless --dry-run
//   SP_RELEASE_DID         repo to publish to (default: stream.place)
//   SP_RELEASE_IDENTIFIER  login identifier (default: SP_RELEASE_DID)
//
// Other flag: --repository URL (source clone URL; default git.stream.place).
//
// Whatever deliverables are in DIR are published; checking that a build is
// complete is the caller's job, since a published version cannot be amended.
//
// Re-running a published version is a no-op if its contents are identical and
// fails if they are not; it never moves a channel back to an old version.
// Schema records are mutable: they are created or updated to match the checkout
// even when the release itself is already published.
import { AtpAgent, AtUri, lexToJson } from "@atproto/api";
import { execFileSync } from "node:child_process";
import { createReadStream } from "node:fs";
import { Readable } from "node:stream";
import { fileURLToPath } from "node:url";
import { parseArgs } from "node:util";
import {
  buildInventory,
  CHANNEL,
  DEFAULT_DID,
  DEFAULT_REPOSITORY,
  gitRevision,
  localBlobRef,
  MANIFEST,
  planRelease,
  PROJECT,
  PROJECTS,
  readSchemaDocs,
  SCHEMA,
  validateInputs,
  VERSION,
} from "./release-lib.mjs";

const USAGE = `usage: release.mjs --artifacts DIR --version VERSION --commit FULL_COMMIT --channel CHANNEL
                  [--changelog URL] [--repository URL] [--dry-run]`;

const repoRoot = fileURLToPath(new URL("../../..", import.meta.url));
const log = (...args) => console.error(...args);

const { values } = (() => {
  try {
    return parseArgs({
      options: {
        artifacts: { type: "string" },
        version: { type: "string" },
        commit: { type: "string" },
        channel: { type: "string" },
        changelog: { type: "string" },
        repository: { type: "string", default: DEFAULT_REPOSITORY },
        "dry-run": { type: "boolean", default: false },
      },
      strict: true,
    });
  } catch (err) {
    console.error(`${err.message}\n${USAGE}`);
    process.exit(2);
  }
})();
for (const required of ["artifacts", "version", "commit", "channel"]) {
  if (!values[required]) {
    console.error(`--${required} is required\n${USAGE}`);
    process.exit(2);
  }
}

/** refs/tags/VERSION, but only if that tag really is the pinned commit. */
function tagRef(version, commit) {
  const ref = `refs/tags/${version}`;
  try {
    const tagged = execFileSync(
      "git",
      ["-C", repoRoot, "rev-parse", "--verify", "--quiet", `${ref}^{commit}`],
      { encoding: "utf8", stdio: ["ignore", "pipe", "ignore"] },
    ).trim();
    if (tagged === commit) return ref;
    log(`${ref} is ${tagged}, not ${commit}; pinning the commit without a ref`);
  } catch {
    log(`no ${ref} in this checkout; pinning the commit without a ref`);
  }
  return undefined;
}

async function getRecord(agent, repo, collection, rkey) {
  try {
    const { data } = await agent.com.atproto.repo.getRecord({
      repo,
      collection,
      rkey,
    });
    return { cid: data.cid, value: lexToJson(data.value) };
  } catch (err) {
    if (err?.error === "RecordNotFound") return undefined;
    throw err;
  }
}

/** Reads the records the plan depends on, in the shape planRelease expects. */
async function loadExisting(
  agent,
  did,
  { version, channel, projects, schemaDocs },
) {
  const existing = { projects: {}, versions: {}, channels: {}, schemas: {} };
  for (const name of PROJECTS) {
    existing.projects[name] = (
      await getRecord(agent, did, PROJECT, name)
    )?.value;
  }
  for (const { name } of projects) {
    const v = await getRecord(agent, did, VERSION, `${name}.${version}`);
    if (v) {
      const ref = v.value.manifest;
      const uri = ref?.uri && new AtUri(ref.uri);
      if (!uri || uri.host !== did || uri.collection !== MANIFEST) {
        throw new Error(
          `${name}.${version} refers to a manifest outside this repo`,
        );
      }
      const m = await getRecord(agent, did, MANIFEST, uri.rkey);
      if (!m || m.cid !== ref.cid) {
        throw new Error(
          `${name}.${version}'s manifest ${ref.uri} is missing or changed`,
        );
      }
      existing.versions[name] = {
        record: v.value,
        cid: v.cid,
        manifest: { record: m.value, cid: m.cid },
      };
    }
    existing.channels[name] = (
      await getRecord(agent, did, CHANNEL, `${name}.${channel}`)
    )?.value;
  }
  await Promise.all(
    schemaDocs.map(async ({ id }) => {
      existing.schemas[id] = (await getRecord(agent, did, SCHEMA, id))?.value;
    }),
  );
  return existing;
}

/** Uploads original bytes without buffering them; the PDS may normalize MIME. */
async function uploadBlob(agent, file) {
  const res = await agent.sessionManager.fetchHandler(
    "/xrpc/com.atproto.repo.uploadBlob",
    {
      method: "POST",
      headers: {
        "content-type": file.mimeType,
        "content-length": String(file.size),
      },
      body: Readable.toWeb(createReadStream(file.path)),
      duplex: "half",
    },
  );
  if (!res.ok) {
    throw new Error(`uploading ${file.fileName}: HTTP ${res.status}`);
  }
  const { blob } = await res.json();
  if (blob?.ref?.$link !== file.cid.toString() || blob.size !== file.size) {
    throw new Error(
      `PDS stored ${file.fileName} as ${blob?.ref?.$link} (${blob?.size} bytes), expected ${file.cid} (${file.size} bytes)`,
    );
  }
  return blob;
}

const mib = (n) => `${(n / 1024 / 1024).toFixed(1)} MiB`;

async function main() {
  const version = values.version;
  const channel = values.channel;
  const commit = values.commit;
  const dryRun = values["dry-run"];
  const did = process.env.SP_RELEASE_DID || DEFAULT_DID;
  validateInputs({ version, channel, commit });
  if (!did.startsWith("did:"))
    throw new Error(`SP_RELEASE_DID ${did} is not a DID`);

  const pds = process.env.SP_RELEASE_PDS;
  const password = process.env.SP_RELEASE_PASSWORD;
  if (!dryRun && (!pds || !password)) {
    throw new Error(
      "SP_RELEASE_PDS and SP_RELEASE_PASSWORD are required (or use --dry-run)",
    );
  }

  const projects = await buildInventory({
    dir: values.artifacts,
    version,
  });
  const schemaDocs = readSchemaDocs();
  const files = projects.flatMap((p) => p.files);
  log(
    `${version}: ${files.length} artifacts in ${projects.map((p) => p.name).join(", ")}, ${schemaDocs.length} lexicon schemas`,
  );

  const source = gitRevision({
    repository: values.repository,
    commit,
    ref: tagRef(version, commit),
  });
  const blobs = new Map(files.map((f) => [f.fileName, localBlobRef(f)]));
  const input = {
    did,
    version,
    channel,
    changelog: values.changelog,
    source,
    projects,
    schemaDocs,
  };
  const emit = (summary) =>
    process.stdout.write(
      `${JSON.stringify({ repo: did, version, channel, commit, ...summary }, null, 2)}\n`,
    );

  if (dryRun) {
    const plan = planRelease({
      ...input,
      blobs,
      existing: { projects: {}, versions: {}, channels: {}, schemas: {} },
    });
    emit({
      dryRun: true,
      note: "assumes an empty repo; blob MIME types are declared, a PDS may normalize them",
      uploads: files.map((f) => ({
        fileName: f.fileName,
        size: f.size,
        cid: f.cid.toString(),
        mimeType: f.mimeType,
      })),
      writes: plan.writes.map(({ collection, rkey, ...w }) => w),
    });
    return;
  }

  const agent = new AtpAgent({ service: pds });
  await agent.login({
    identifier: process.env.SP_RELEASE_IDENTIFIER || did,
    password,
  });
  if (agent.did !== did) {
    throw new Error(`logged in as ${agent.did}, but SP_RELEASE_DID is ${did}`);
  }

  // Check immutable records before uploading any artifact.
  const preflight = planRelease({
    ...input,
    blobs,
    existing: await loadExisting(agent, did, {
      version,
      channel,
      projects,
      schemaDocs,
    }),
  });
  if (preflight.writes.length === 0) {
    for (const note of preflight.notes) log(note);
    log("nothing to publish");
    emit({
      dryRun: false,
      published: false,
      notes: preflight.notes,
      writes: [],
    });
    return;
  }

  const uploads = projects
    .filter((p) => preflight.created.includes(p.name))
    .flatMap((p) => p.files);

  // Servers that advertise a blob size limit are checked before any upload.
  const { data: server } = await agent.com.atproto.server.describeServer();
  const limit = server.blobUploadLimit;
  const tooBig = uploads.filter((f) => limit !== undefined && f.size > limit);
  if (tooBig.length > 0) {
    throw new Error(
      `the PDS accepts blobs up to ${mib(limit)}, but ${tooBig.map((f) => `${f.fileName} is ${mib(f.size)}`).join(", ")}; raise PDS_BLOB_UPLOAD_LIMIT`,
    );
  }

  for (const file of uploads) {
    log(`uploading ${file.fileName} (${mib(file.size)})`);
    blobs.set(file.fileName, await uploadBlob(agent, file));
  }

  // Plan again against the repo as it is now; the commit swap below makes the
  // write fail rather than land on state this plan did not see.
  const { data: head } = await agent.com.atproto.sync.getLatestCommit({ did });
  const plan = planRelease({
    ...input,
    blobs,
    existing: await loadExisting(agent, did, {
      version,
      channel,
      projects,
      schemaDocs,
    }),
  });
  if (plan.created.some((name) => !preflight.created.includes(name))) {
    throw new Error("the repo changed while uploading; run again");
  }
  if (plan.writes.length === 0) {
    emit({ dryRun: false, published: false, notes: plan.notes, writes: [] });
    return;
  }
  let result;
  try {
    ({ data: result } = await agent.com.atproto.repo.applyWrites({
      repo: did,
      swapCommit: head.cid,
      writes: plan.writes.map(({ action, collection, rkey, value }) => ({
        $type: `com.atproto.repo.applyWrites#${action}`,
        collection,
        rkey,
        value,
      })),
    }));
  } catch (err) {
    if (err?.error === "InvalidSwap") {
      throw new Error("the repo changed while publishing; run again");
    }
    throw err;
  }
  plan.writes.forEach((w, i) => {
    const got = result.results?.[i];
    if (got?.uri !== w.uri || got?.cid !== w.cid) {
      throw new Error(
        `PDS wrote ${got?.uri} as ${got?.cid}, expected ${w.uri} as ${w.cid}; the release records are inconsistent`,
      );
    }
  });
  for (const note of plan.notes) log(note);
  log(`published ${version} to ${did} (${plan.writes.length} records)`);
  emit({
    dryRun: false,
    published: true,
    notes: plan.notes,
    writes: plan.writes.map(({ action, uri, cid }) => ({ action, uri, cid })),
  });
}

main().catch((err) => {
  console.error(`release: ${err.message}`);
  process.exit(1);
});
