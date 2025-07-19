import { v7 as uuidv7 } from "uuid";
import { makeWindow } from "../window";
import { E2ETest, TestEnv } from "./test-env";
import { delay, PlayerReport } from "./util";

/**
 * This test:
 * - Plays a stream that goes up/down every 15 seconds
 * - Observes the player for several cycles
 * - Checks that the player spends a reasonable amount of time in 'playing' state after each recovery
 */

const PLAYING_THRESHOLD = 0.4;

export const resumeLoopTest: E2ETest = {
  setup: async (testEnv: TestEnv) => {
    return {
      ...testEnv,
      env: {
        ...testEnv.env,
        SP_TEST_STREAM: "true",
      },
    };
  },
  test: async (testEnv: TestEnv): Promise<string | null> => {
    const mainWindow = await makeWindow();

    const testId = uuidv7();
    const playerId = `${testId}-intermittent`;
    const playerConfig = {
      name: "intermittent-stream",
      playerId,
      src: "intermittent-self-test", // <-- Make sure this matches your Go alias!
      showControls: true,
      telemetry: true,
      forceProtocol: "webrtc",
    };
    const enc = encodeURIComponent(JSON.stringify([playerConfig]));
    const load = `${testEnv.addr}/multi/${enc}`;

    console.log(`Opening player at ${load}`);
    await mainWindow.loadURL(load);

    await delay(testEnv.testDuration);

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
      return `Player spent too little time playing during intermittent stream (${(
        playingPct * 100
      ).toFixed(1)}%). Possible stall or failure to recover.`;
    }

    return null;
  },
};
