import { bytesToMultibase } from "@atproto/crypto";
import { ChildProcess } from "child_process";
import fs from "fs/promises";
import os from "os";
import path from "path";
import { privateKeyToAccount } from "viem/accounts";
import getEnv from "../env";
import makeNode from "../node";
import { playbackTest } from "./playback-test";
import { resumeLoopTest } from "./resume-loop-test";
import { syncTest } from "./sync-test";
import { E2ETest, TestEnv } from "./test-env";

const allTests: Record<string, E2ETest> = {
  playback: playbackTest,
  sync: syncTest,
  resume: resumeLoopTest,
};

export const allTestNames = Object.keys(allTests);

const randomPort = () => Math.floor(Math.random() * 20000) + 20000;

export default async function runTests(
  tests: string[],
  duration: string,
  privateKey: `0x${string}`,
): Promise<boolean> {
  const testFuncs = [];
  for (const test of tests) {
    if (!allTests[test]) {
      throw new Error(`Test ${test} not found`);
    }
    testFuncs.push(allTests[test]);
  }
  try {
    const results = [];
    for (const test of testFuncs) {
      let testProc: ChildProcess | undefined;
      try {
        const { skipNode } = getEnv();
        const hexKey = privateKey.slice(2); // Remove 0x prefix
        const exportedKey = new Uint8Array(
          hexKey.match(/.{1,2}/g).map((byte) => parseInt(byte, 16)),
        );
        const multibaseKey = bytesToMultibase(exportedKey, "base58btc");
        const account = privateKeyToAccount(privateKey);
        const tmpDir = await fs.mkdtemp(
          path.join(os.tmpdir(), "streamplace-test-"),
        );

        let testEnv: TestEnv = {
          addr: "http://127.0.0.1:38080",
          internalAddr: "http://127.0.0.1:39090",
          privateKey: privateKey,
          publicAddress: account.address.toLowerCase(),
          testDuration: parseInt(duration),
          multibaseKey,
          env: {},
        };
        if (!skipNode) {
          testEnv.env = {
            SP_HTTP_ADDR: `127.0.0.1:${randomPort()}`,
            SP_HTTP_INTERNAL_ADDR: `127.0.0.1:${randomPort()}`,
            SP_DATA_DIR: tmpDir,
          };
        }
        if (test.setup) {
          testEnv = await test.setup(testEnv);
        }
        if (!skipNode) {
          const { addr, internalAddr, proc } = await makeNode({
            env: testEnv.env,
            autoQuit: false,
          });
          testEnv.addr = addr;
          testEnv.internalAddr = internalAddr;
          testProc = proc;
        }
        const result = await test.test(testEnv);
        results.push(result);
      } catch (e) {
        console.error("error running test", e.message);
        results.push(e.message);
      } finally {
        if (testProc) {
          testProc.kill("SIGTERM");
        }
      }
    }
    const failures = results.filter((r) => r !== null);
    if (failures.length > 0) {
      console.error("tests failed", failures.join(", "));
      return false;
    }
    return true;
  } catch (e) {
    console.error("error running tests", e.message);
    return false;
  }
}
