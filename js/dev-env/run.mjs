import { access } from "node:fs/promises";
import { createRequire } from "node:module";
import { TestNetwork } from "./dist/index.js";
import { TestVlpdsServer } from "./dist/vlpds.js";

// The HTTPS harness terminates TLS in front of the plain-HTTP listener.
const hostname = process.env.DEV_ENV_PDS_HOSTNAME;
const flavor = process.env.DEV_ENV_PDS || "reference";
const lexiconPassword = "lexicons";
if (!["reference", "spaces", "vlpds"].includes(flavor)) {
  throw new Error(`unknown DEV_ENV_PDS flavor: ${flavor}`);
}

let implementation;
if (flavor === "spaces") {
  // The alpha PDS is an optional, isolated installation.
  try {
    await access(
      new URL(
        "./spaces-pds/node_modules/@atproto/pds/package.json",
        import.meta.url,
      ),
    );
    const require = createRequire(
      new URL("./spaces-pds/package.json", import.meta.url),
    );
    implementation = require("@atproto/pds");
  } catch (cause) {
    throw new Error("spaces PDS unavailable; run hack/fetch-spaces-pds.sh", {
      cause,
    });
  }
}

let network;
let vlpds;
let stopping = false;
async function close() {
  if (stopping) return;
  stopping = true;
  await vlpds?.close();
  await network?.close();
}
for (const signal of ["SIGINT", "SIGTERM"]) {
  process.once(signal, () => {
    close().then(
      () => process.exit(0),
      (err) => {
        console.error(err);
        process.exit(1);
      },
    );
  });
}

try {
  network = await TestNetwork.create({
    pds: {
      ...(implementation ? { implementation } : {}),
      ...(hostname && flavor !== "vlpds"
        ? {
            hostname,
            // .test handles intentionally fail public verification in tests.
            serviceHandleDomains: [`.${hostname}`, ".test"],
          }
        : {}),
    },
  });
  const out = {
    "pds-url": network.pds.url,
    "plc-url": network.plc.url,
    "lexicon-pds-url": network.pds.url,
  };
  if (hostname || flavor !== "reference") {
    const lexiconDomain = flavor === "vlpds" ? "test" : hostname || "test";
    const res = await fetch(
      `${network.pds.url}/xrpc/com.atproto.server.createAccount`,
      {
        method: "POST",
        headers: { "content-type": "application/json" },
        body: JSON.stringify({
          handle: `lexicons.${lexiconDomain}`,
          email: `lexicons@${hostname || "example.com"}`,
          password: lexiconPassword,
        }),
      },
    );
    if (!res.ok) {
      throw new Error(`create lexicon account: ${await res.text()}`);
    }
    const { did } = await res.json();
    // Both reference PDS versions read this config on each lexicon lookup.
    network.pds.ctx.cfg.lexicon.didAuthority = did;
    out["lexicon-did"] = did;
    out["lexicon-password"] = lexiconPassword;
    if (flavor === "vlpds") {
      vlpds = await TestVlpdsServer.create(hostname, network.plc.url, did);
      vlpds.process.once("exit", (code, signal) => {
        if (!stopping) {
          console.error(`vlpds exited unexpectedly (${code ?? signal})`);
          close().finally(() => process.exit(1));
        }
      });
      out["pds-url"] = vlpds.url;
    }
  }
  console.log(JSON.stringify(out));
} catch (err) {
  console.error(err);
  await close();
  process.exit(1);
}
