import { spawn, type ChildProcess } from "node:child_process";
import { constants } from "node:fs";
import { access } from "node:fs/promises";
import { setTimeout } from "node:timers/promises";
import { fileURLToPath } from "node:url";

import { Secp256k1Keypair } from "@atproto/crypto";
import getPort from "get-port";
import * as ui8 from "uint8arrays";

export class TestVlpdsServer {
  private constructor(
    public readonly process: ChildProcess,
    public readonly url: string,
  ) {}

  static async create(
    hostname: string | undefined,
    plcURL: string,
    lexiconDID: string,
  ) {
    const binary = fileURLToPath(
      new URL("../../../build-linux-amd64/vlpds", import.meta.url),
    );
    try {
      await access(binary, constants.X_OK);
    } catch {
      throw new Error(
        `vlpds binary missing at ${binary}; run hack/fetch-vlpds.sh on the Docker host`,
      );
    }
    const port = await getPort();
    const publicURL = hostname
      ? `https://${hostname}`
      : `http://localhost:${port}`;
    const key = await Secp256k1Keypair.create({ exportable: true });
    const env = { ...process.env };
    delete env.VLPDS_UI_DIR;
    if (env.NODE_EXTRA_CA_CERTS) env.SSL_CERT_FILE = env.NODE_EXTRA_CA_CERTS;
    const child = spawn(
      binary,
      [
        "--memory",
        "--dev-mode",
        "--spaces",
        "--no-rate-limits",
        "--listen",
        `127.0.0.1:${port}`,
        "--public-url",
        publicURL,
        "--handle-domain",
        hostname || "test",
        "--service-did",
        `did:web:${new URL(publicURL).host.replace(":", "%3A")}`,
        "--plc-url",
        plcURL,
        "--plc-mode",
        "directory",
        "--plc-rotation-key",
        ui8.toString(await key.export(), "hex"),
        "--lexicon-authority-override",
        `stream.place=${lexiconDID}`,
      ],
      { env, stdio: ["ignore", process.stderr, process.stderr] },
    );
    const server = new TestVlpdsServer(child, `http://127.0.0.1:${port}`);
    const onExit = () => child.kill("SIGKILL");
    process.once("exit", onExit);
    child.once("exit", () => process.removeListener("exit", onExit));
    let startupError: Error | undefined;
    child.once("error", (err) => {
      startupError = err;
    });
    try {
      const deadline = Date.now() + 30_000;
      while (Date.now() < deadline) {
        if (startupError) throw startupError;
        if (child.exitCode !== null || child.signalCode !== null) {
          throw new Error(
            `vlpds exited before startup (${child.exitCode ?? child.signalCode})`,
          );
        }
        try {
          const res = await fetch(
            `${server.url}/xrpc/com.atproto.server.describeServer`,
            { signal: AbortSignal.timeout(Math.max(1, deadline - Date.now())) },
          );
          if (res.ok) return server;
        } catch {
          // The child has not bound its listener yet.
        }
        await setTimeout(100);
      }
      throw new Error("vlpds did not become ready within 30 seconds");
    } catch (err) {
      await server.close();
      throw err;
    }
  }

  async close() {
    const child = this.process;
    if (!child.pid || child.exitCode !== null || child.signalCode !== null)
      return;
    const { promise, resolve } = Promise.withResolvers<void>();
    const timer = globalThis.setTimeout(() => child.kill("SIGKILL"), 5_000);
    child.once("exit", () => {
      clearTimeout(timer);
      resolve();
    });
    child.kill("SIGTERM");
    await promise;
  }
}
