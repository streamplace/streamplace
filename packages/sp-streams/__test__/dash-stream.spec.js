import { killall } from "../src/ffmpeg";
import path from "path";
import dashStream from "../src/dash-stream";
import fs from "fs";
import { parseString } from "xml2js";

const TEST_FILE_PATH = path.resolve(__dirname, "elis-face.ts");
// const TEST_FILE_PATH = "/Content/EliStreams/2017-07-25 13-19-36.ts";

describe("dashStream", () => {
  let dash;
  afterEach(() => {
    killall();
    dash.end();
  });

  it("should give us chunks", done => {
    const fileData = fs.readFileSync(TEST_FILE_PATH);
    dash = dashStream();
    let manifestCount = 0;
    let chunkCount = 0;
    dash.push(fileData);
    const checkDone = () => {
      if (manifestCount !== 5 || chunkCount !== 10) {
        return;
      }
      done();
    };

    dash.on("manifest", manifest => {
      manifestCount += 1;
      parseString(manifest, err => {
        expect(err).toBeNull();
      });
      checkDone();
    });
    dash.on("chunk", (filename, stream) => {
      stream.resume();
      chunkCount += 1;
      checkDone();
    });
  });
});
