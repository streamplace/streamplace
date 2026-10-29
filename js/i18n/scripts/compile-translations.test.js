const assert = require("node:assert/strict");
const { execFile } = require("node:child_process");
const fs = require("node:fs");
const os = require("node:os");
const path = require("node:path");
const { setImmediate } = require("node:timers/promises");
const { test } = require("node:test");
const { promisify } = require("node:util");

const exec = promisify(execFile);

function fixture(t) {
  const root = fs.mkdtempSync(path.join(os.tmpdir(), "streamplace-i18n-"));
  t.after(() => fs.rmSync(root, { recursive: true, force: true }));
  const i18n = path.join(root, "js/i18n");
  fs.mkdirSync(path.join(i18n, "scripts"), { recursive: true });
  fs.copyFileSync(
    path.join(__dirname, "compile-translations.js"),
    path.join(i18n, "scripts/compile-translations.js"),
  );
  fs.cpSync(path.join(__dirname, "../locales"), path.join(i18n, "locales"), {
    recursive: true,
  });
  fs.symlinkSync(
    path.resolve(__dirname, "../../../node_modules"),
    path.join(root, "node_modules"),
    "dir",
  );
  fs.mkdirSync(path.join(root, "js/app/public"), { recursive: true });
  return root;
}

async function compile(root) {
  const { stderr } = await exec(process.execPath, [
    path.join(root, "js/i18n/scripts/compile-translations.js"),
  ]);
  assert.equal(stderr, "");
}

test("generates translations and updates existing consumer files", async (t) => {
  const root = fixture(t);
  await compile(root);
  assert.equal(fs.existsSync(path.join(root, "js/web/public")), false);
  const locale = "locales/en-US/common.json";
  const initial = fs.readFileSync(
    path.join(root, "js/i18n/public", locale),
    "utf8",
  );
  assert.deepEqual(
    JSON.parse(
      fs.readFileSync(path.join(root, "js/app/public", locale), "utf8"),
    ),
    JSON.parse(initial),
  );

  fs.appendFileSync(
    path.join(root, "js/i18n/locales/en-US/common.ftl"),
    "\ncompile-regression = updated\n",
  );
  fs.mkdirSync(path.join(root, "js/web/public"), { recursive: true });
  await compile(root);
  for (const consumer of ["i18n", "app", "web"]) {
    const output = JSON.parse(
      fs.readFileSync(path.join(root, `js/${consumer}/public`, locale), "utf8"),
    );
    assert.deepEqual(output, {
      ...JSON.parse(initial),
      "compile-regression": "updated",
    });
  }
});

test("keeps generated JSON readable during overlapping compilations", async (t) => {
  const root = fixture(t);
  fs.mkdirSync(path.join(root, "js/web/public"), { recursive: true });
  await compile(root);
  const outputDir = path.join(root, "js/i18n/public/locales");
  const entries = fs.readdirSync(outputDir, { recursive: true }).sort();
  const expected = entries
    .filter((file) => file.endsWith(".json"))
    .map((file) => [
      file,
      JSON.parse(fs.readFileSync(path.join(outputDir, file), "utf8")),
    ]);
  function assertOutputs() {
    for (const consumer of ["i18n", "app", "web"]) {
      assert.deepEqual(
        fs
          .readdirSync(path.join(root, `js/${consumer}/public/locales`), {
            recursive: true,
          })
          .sort(),
        entries,
      );
      for (const [file, messages] of expected) {
        const output = path.join(root, `js/${consumer}/public/locales`, file);
        assert.deepEqual(JSON.parse(fs.readFileSync(output, "utf8")), messages);
      }
    }
  }
  let running = true;
  const builds = Promise.allSettled(
    Array.from({ length: 4 }, () => compile(root)),
  ).finally(() => {
    running = false;
  });
  let readError;
  try {
    do {
      assertOutputs();
      await setImmediate();
    } while (running);
  } catch (error) {
    readError = error;
  }
  for (const result of await builds) {
    if (result.status === "rejected") throw result.reason;
  }
  if (readError) throw readError;
  assertOutputs();
});

test("reports output failures without leaving temporary files", async (t) => {
  const root = fixture(t);
  const outputDir = path.join(root, "js/app/public/locales/en-US");
  fs.mkdirSync(path.join(outputDir, "common.json"), { recursive: true });
  await assert.rejects(compile(root), (error) => {
    assert.equal(error.code, 1);
    assert.match(error.stderr, /Error processing .*common\.ftl/);
    return true;
  });
  assert.deepEqual(fs.readdirSync(outputDir), ["common.json"]);
  assert.deepEqual(fs.readdirSync(path.join(root, ".build")), []);
});
