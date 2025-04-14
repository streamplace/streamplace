import { delay, PlayerReport } from "./util";
import { v7 as uuidv7 } from "uuid";
import { makeWindow } from "../window";
import { E2ETest, TestEnv } from "./test-env";

const PLAYING_SUCCESS = 0.5;

export const playbackTest: E2ETest = {
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
    const definitions = [
      // {
      //   name: "hls",
      //   forceProtocol: "hls",
      // },
      {
        name: "progressive-mp4",
        forceProtocol: "progressive-mp4",
      },
      {
        name: "progressive-webm",
        forceProtocol: "progressive-webm",
      },
      {
        name: "webrtc",
        forceProtocol: "webrtc",
      },
    ];
    const tests = definitions.map((x) => ({
      name: x.name,
      playerId: `${testId}-${x.name}`,
      src: "self-test",
      showControls: true,
      telemetry: true,
      forceProtocol: x.forceProtocol,
    }));
    const enc = encodeURIComponent(JSON.stringify(tests));

    const load = `${testEnv.addr}/multi/${enc}`;

    console.log(`opening ${load}`);
    mainWindow.loadURL(load);

    let foundThumbnail = false;
    const interval = setInterval(async () => {
      const thumb = `${testEnv.addr}/api/playback/self-test/stream.jpg`;
      console.log(`fetching thumbnail at ${thumb}`);
      const res = await fetch(thumb);
      if (res.status === 404) {
        console.log("no thumbnail found");
        return;
      }
      if (res.status !== 200) {
        console.log(
          `unexpected http status ${res.status}, failing thumbnail test`,
        );
        clearInterval(interval);
        return;
      }
      const blob = await res.arrayBuffer();
      if (blob.byteLength < 1) {
        console.log("thumbnail was empty :(");
        return;
      }
      console.log("found thumbnail!");
      foundThumbnail = true;
      clearInterval(interval);
    }, 1000);

    await delay(testEnv.testDuration);
    clearInterval(interval);
    const reports = await Promise.all(
      tests.map(async (t) => {
        const res = await fetch(
          `${testEnv.internalAddr}/player-report/${t.playerId}`,
        );
        const data = (await res.json()) as PlayerReport;
        return { ...t, data: data.whatHappened, retries: data.retries };
      }),
    );
    let failed = false;
    if (!foundThumbnail) {
      console.log("never found a thumbnail, failing test");
      failed = true;
    }
    const percentages = reports.map((report) => {
      if (typeof report.retries === "number" && report.retries > 1) {
        console.log(`${report.name} had ${report.retries} retries`);
        // we only care about webrtc failures right now
        if (report.name === "webrtc") {
          failed = true;
        }
      }
      let total = 0;
      for (const [state, ms] of Object.entries(report.data)) {
        total += ms;
      }
      const pcts: { [k: string]: number } = { playing: 0 };
      for (const [state, ms] of Object.entries(report.data)) {
        pcts[state] = ms / total;
      }
      if (pcts.playing < PLAYING_SUCCESS) {
        failed = true;
      }
      return { ...report, pcts };
    });
    console.log(JSON.stringify(percentages, null, 2));
    await mainWindow.close();
    if (failed) {
      return "test failed!";
    }
    return null;
  },
};
