import { killall } from "../src/ffmpeg";
import path from "path";
import dashStream from "../src/dash-stream";
import fs from "fs";

const TEST_FILE_PATH = path.resolve(__dirname, "elis-face.ts");
// const TEST_FILE_PATH = "/Content/EliStreams/2017-07-25 13-19-36.ts";

describe("dashStream", () => {
  let dash;
  afterEach(() => {
    killall();
    dash.end();
  });

  it("should give us chunks", done => {
    const fileInput = fs.createReadStream(TEST_FILE_PATH);
    dash = dashStream();
    fileInput.pipe(dash, { end: false });
    let manifestCount = 0;
    let chunkCount = 0;

    const checkDone = () => {
      if (manifestCount !== 5 || chunkCount !== 10) {
        return;
      }
      done();
    };

    dash.on("manifest", manifest => {
      manifestCount += 1;
      checkDone();
    });
    dash.on("chunk", (filename, stream) => {
      stream.resume();
      chunkCount += 1;
      checkDone();
    });
  });
});
