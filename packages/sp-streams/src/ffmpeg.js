import fluent from "fluent-ffmpeg";
import debug from "debug";

const log = debug("sp:ffmpeg");

let allFfmpegs = [];
export function killall() {
  allFfmpegs.forEach(proc => proc.kill());
  allFfmpegs = [];
}

export default function ffmpeg() {
  let logCounter = 0;
  const thisFfmpeg = fluent()
    .on("error", (err, stdout, stderr) => {
      if (err.toString() === "Error: ffmpeg was killed with signal SIGKILL") {
        return;
      }
      log("ffmpeg error", { err: err.toString(), stdout, stderr });
    })
    .on("codecData", data => {
      log("ffmpeg codecData", data);
    })
    .on("end", () => {
      log("ffmpeg end");
      allFfmpegs = allFfmpegs.filter(x => x !== thisFfmpeg);
    })
    .on("progress", data => {
      if (logCounter === 0) {
        log(`[${data.timemark}] ${data.currentFps}FPS ${data.currentKbps}Kbps`);
      }
      logCounter = (logCounter + 1) % 15;
    })
    .on("start", command => {
      const sanitizedCommand = command;
      log("ffmpeg start: " + sanitizedCommand);
      // instance.ffmpegProc.stdout.on("data", data => {
      //   log(data.toString());
      // });
      // instance.ffmpegProc.stderr.on("data", data => {
      //   log(data.toString());
      // });
    });
  allFfmpegs.push(thisFfmpeg);
  return thisFfmpeg;
}
