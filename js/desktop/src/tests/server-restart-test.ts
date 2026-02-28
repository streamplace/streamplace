import fs from "fs/promises";
import os from "os";
import path from "path";
import { v7 as uuidv7 } from "uuid";
import makeNode from "../node";
import { makeWindow } from "../window";
import { E2ETest, TestEnv } from "./test-env";
import { delay, PlayerReport, randomPort } from "./util";

/**
 * This test:
 * - Starts a new Streamplace node
 * - Starts playing
 * - Crashes that node and restarts it
 * - Make sure we're still playing again afterwards
 */

const PLAYING_THRESHOLD = 0.5;

export const serverRestartTest: E2ETest = {
  test: async (testEnv: TestEnv): Promise<string | null> => {
    const mainWindow = await makeWindow();

    const tmpDir = await fs.mkdtemp(
      path.join(os.tmpdir(), "streamplace-test-"),
    );

    // this test runs another node so we can still use node 1 for reporting!
    const env = {
      SP_HTTP_ADDR: `127.0.0.1:${randomPort()}`,
      SP_HTTP_INTERNAL_ADDR: `127.0.0.1:${randomPort()}`,
      SP_RTMP_ADDR: `127.0.0.1:${randomPort()}`,
      SP_DATA_DIR: tmpDir,
      SP_TEST_STREAM: "true",
      SP_NO_FIREHOSE: "true",
      SP_WIDE_OPEN: "true",
    };
    let { proc } = await makeNode({
      env: env,
      autoQuit: false,
    });

    const testId = uuidv7();
    const playerId = `${testId}-server-restart`;
    const playerConfig = {
      name: "server-restart-stream",
      playerId,
      src: "self-test", // <-- Make sure this matches your Go alias!
      showControls: true,
      telemetry: true,
      forceProtocol: "webrtc",
      reportingURL: `${testEnv.addr}/api/player-event`,
    };
    const enc = encodeURIComponent(JSON.stringify([playerConfig]));
    const load = `http://${env.SP_HTTP_ADDR}/multi/${enc}`;

    console.log(`Opening player at ${load}`);
    await mainWindow.loadURL(load);

    await delay(20000);
    proc.kill("SIGKILL");
    await delay(500);
    let { proc: proc2 } = await makeNode({
      env: env,
      autoQuit: false,
    });

    await delay(60000);
    proc2.kill("SIGTERM");

    const res = await fetch(
      `${testEnv.internalAddr}/player-report/${playerId}`,
    );
    const data = (await res.json()) as PlayerReport;

    const stateTimes = data.whatHappened || {};
    const total = Object.values(stateTimes).reduce((a, b) => a + b, 0);
    const playing = stateTimes["playing"] || 0;

    const playingPct = total > 0 ? playing / total : 0;

    console.log(
      `Overall playing percentage: ${(playingPct * 100).toFixed(1)}%`,
    );
    console.log("Full state times:", JSON.stringify(stateTimes, null, 2));

    mainWindow.close();

    if (playingPct < PLAYING_THRESHOLD) {
      return `Player spent too little time playing during server-restart stream (${(
        playingPct * 100
      ).toFixed(1)}%). Possible stall or failure to recover.`;
    }
    proc2.kill("SIGTERM");

    return null;
  },
};
