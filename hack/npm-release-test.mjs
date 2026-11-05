#!/usr/bin/env node

// Dry run of `make ci-npm-release`. Builds and packs every package that
// `lerna publish` would release, then checks the tarballs as a consumer would
// see them: every workspace dependency must be published alongside the
// package, and each entry point (both the JavaScript `default` export and the
// `types` entry that TypeScript consumers check against) must bundle, with web
// and native file resolution, from the unpacked tarballs alone.

import * as esbuild from "esbuild";
import { execFileSync } from "node:child_process";
import {
  mkdirSync,
  mkdtempSync,
  readdirSync,
  readFileSync,
  rmSync,
} from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { fileURLToPath } from "node:url";

const repo = fileURLToPath(new URL("..", import.meta.url));

function run(cmd, args, cwd) {
  return execFileSync(cmd, args, {
    cwd,
    encoding: "utf8",
    stdio: ["ignore", "pipe", "inherit"],
  });
}

function lernaLs(...flags) {
  const out = run(
    "pnpm",
    ["exec", "lerna", "ls", "--json", "--loglevel", "silent", ...flags],
    repo,
  );
  return JSON.parse(out);
}

// "@scope/name/deep/path" -> "@scope/name"; "name/deep" -> "name"
function packageName(specifier) {
  const parts = specifier.split("/");
  return parts.slice(0, specifier.startsWith("@") ? 2 : 1).join("/");
}

const published = new Map(lernaLs().map((p) => [p.name, p]));
const workspace = new Set(lernaLs("--all").map((p) => p.name));
const dir = mkdtempSync(join(tmpdir(), "npm-release-test-"));
const modules = join(dir, "node_modules");
const errors = [];

try {
  for (const pkg of published.values()) {
    run("pnpm", ["run", "--if-present", "prepare"], pkg.location);
    const packDir = join(dir, "packs", pkg.name);
    mkdirSync(packDir, { recursive: true });
    run("pnpm", ["pack", "--pack-destination", packDir], pkg.location);
    const root = join(modules, pkg.name);
    mkdirSync(root, { recursive: true });
    const [tarball] = readdirSync(packDir);
    run("tar", [
      "xzf",
      join(packDir, tarball),
      "-C",
      root,
      "--strip-components=1",
    ]);

    const manifest = JSON.parse(
      readFileSync(join(root, "package.json"), "utf8"),
    );
    for (const [dep, range] of Object.entries({
      ...manifest.optionalDependencies,
      ...manifest.peerDependencies,
      ...manifest.dependencies,
    })) {
      if (workspace.has(dep) && published.get(dep)?.version !== range) {
        errors.push(
          `${pkg.name} depends on ${dep}@${range}, which is not published alongside it`,
        );
      }
    }
  }

  // Published workspace packages resolve from the tarballs; everything else is
  // a third-party dependency the consumer installs, so it stays external.
  const tarballsOnly = {
    name: "tarballs-only",
    setup(build) {
      build.onResolve({ filter: /^[^./]/ }, (args) =>
        args.kind === "entry-point" || published.has(packageName(args.path))
          ? undefined
          : { external: true },
      );
    },
  };

  for (const name of published.keys()) {
    for (const platform of ["web", "native"]) {
      for (const entry of ["default", "types"]) {
        try {
          await esbuild.build({
            absWorkingDir: dir,
            entryPoints: [name],
            bundle: true,
            write: false,
            logLevel: "silent",
            platform: "neutral",
            conditions: entry === "types" ? ["types"] : [],
            mainFields: entry === "types" ? ["types", "main"] : ["main"],
            resolveExtensions: [
              `.${platform}.tsx`,
              `.${platform}.ts`,
              `.${platform}.js`,
              ".tsx",
              ".ts",
              ".js",
              ".json",
            ],
            loader: { ".png": "empty" },
            plugins: [tarballsOnly],
          });
        } catch (e) {
          if (!e.errors) throw e;
          for (const { text, location } of e.errors) {
            const at = location ? ` (${location.file}:${location.line})` : "";
            errors.push(`${name} [${platform} ${entry}]: ${text}${at}`);
          }
        }
      }
    }
  }
} finally {
  rmSync(dir, { recursive: true, force: true });
}

if (errors.length > 0) {
  console.error(errors.join("\n"));
  process.exit(1);
}
console.log(`npm release dry run ok: ${[...published.keys()].join(", ")}`);
