import { BrowserWindow, WebFrameMain, webFrameMain, session } from "electron";
import { MAIN_WINDOW_PRELOAD_WEBPACK_ENTRY } from "../env";
import { delay, PlayerReport } from "./util";
import { E2ETest, TestEnv } from "./test-env";
import { v7 as uuidv7 } from "uuid";

const SYNC_TOO_FAR = 20000;

export const syncTest: E2ETest = {
  test: async (testEnv: TestEnv): Promise<string | null> => {
    const playerId = `${uuidv7()}-sync-test`;
    const window = new BrowserWindow({
      height: 720,
      width: 1280,
      webPreferences: {
        preload: MAIN_WINDOW_PRELOAD_WEBPACK_ENTRY,
      },
      title: "aquareum-sync-test",
      // titleBarStyle: "hidden",
      // titleBarOverlay: true,
    });
    let frame: WebFrameMain | undefined;
    await new Promise<void>((resolve) => {
      window.webContents.on(
        "did-frame-navigate",
        (
          event,
          url,
          httpResponseCode,
          httpStatusText,
          isMainFrame,
          frameProcessId,
          frameRoutingId,
        ) => {
          frame = webFrameMain.fromId(frameProcessId, frameRoutingId);
          resolve();
        },
      );
      window.loadURL(`${testEnv.addr}/sync-test`);
    });
    session.defaultSession.setDisplayMediaRequestHandler(
      async (request, callback) => {
        callback({ video: frame, audio: frame });
      },
    );
    const streamWindow = new BrowserWindow({
      height: 720,
      width: 1280,
      x: 0,
      y: 0,
      webPreferences: {
        preload: MAIN_WINDOW_PRELOAD_WEBPACK_ENTRY,
      },
      title: "aquareum-sync-stream",
      // titleBarStyle: "hidden",
      // titleBarOverlay: true,
    });
    const streamParams = new URLSearchParams({
      ingestMediaSource: "display",
      ingestStreamKey: testEnv.multibaseKey,
      ingestAutoStart: "true",
    });
    streamWindow.loadURL(
      `${testEnv.addr}/live/webcam?${streamParams.toString()}`,
    );
    const playbackParams = new URLSearchParams({
      avSyncTest: "true",
      playerId: playerId,
    });
    const playbackWindow = new BrowserWindow({
      height: 720,
      width: 1280,
      x: 0,
      y: 0,
      webPreferences: {
        preload: MAIN_WINDOW_PRELOAD_WEBPACK_ENTRY,
      },
      title: "aquareum-sync-playback",
      // titleBarStyle: "hidden",
      // titleBarOverlay: true,
    });
    playbackWindow.loadURL(
      `${testEnv.addr}/${testEnv.publicAddress}?${playbackParams.toString()}`,
    );
    await delay(testEnv.testDuration);
    await Promise.all([
      streamWindow.close(),
      playbackWindow.close(),
      window.close(),
    ]);
    const res = await fetch(
      `${testEnv.internalAddr}/player-report/${playerId}`,
    );
    const data = (await res.json()) as PlayerReport;
    if (!data.avSync) {
      return "av sync not present in output";
    }
    console.log(JSON.stringify(data.avSync));
    let problems = [];
    for (const f of ["min", "max", "avg"] as const) {
      if (Math.abs(data.avSync[f]) > SYNC_TOO_FAR) {
        problems.push(`av sync ${f} is ${data.avSync[f]}`);
      }
    }
    if (problems.length > 0) {
      return problems.join(", ");
    }
    return null;
  },
};
