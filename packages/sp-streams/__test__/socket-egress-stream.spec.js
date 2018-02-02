import ffmpegCreate, { killall, framehash } from "../src/ffmpeg";
import path from "path";
import socketEgressStream from "../src/socket-egress-stream";
import fs from "fs-extra";
import tmp from "tmp-promise";

const TEST_FILE_PATH = path.resolve(__dirname, "elis-face.ts");

describe("socketEgressStream", () => {
  afterEach(() => {
    killall();
  });

  it(
    "should pass data through ffmpeg unharmed",
    () => {
      const socketEgress = socketEgressStream();
      let oldHash;
      let tmpPath;
      return framehash(TEST_FILE_PATH)
        .then(_oldHash => {
          oldHash = _oldHash;
          return tmp.file();
        })
        .then(o => {
          tmpPath = o.path;
          return fs.readFile(TEST_FILE_PATH);
        })
        .then(data => {
          const ffmpeg = ffmpegCreate()
            .input(`unix://${socketEgress.path}`)
            .inputFormat("mpegts")
            .videoCodec("copy")
            .audioCodec("copy")
            .outputOptions(["-vsync passthrough"])
            .output(tmpPath)
            .outputFormat("mpegts");
          ffmpeg.run();
          let length = 0;
          // let start = Date.now();
          // socketIngress.on("data", chunk => {
          //   length += chunk.length;
          //   console.log(length, data.length);
          //   console.log(`${Date.now() - start}ms`);
          // });
          // socketIngress.pipe(fs.createWriteStream(tmpPath));
          socketEgress.push(data);
          socketEgress.end();
          // hacky but i'm just too many layers in here
          return new Promise(resolve => {
            setTimeout(resolve, 1000);
          });
        })
        .then(() => {
          return framehash(tmpPath);
        })
        .then(newHash => {
          expect(newHash).toEqual(oldHash);
        });
    },
    10000
  );
});
