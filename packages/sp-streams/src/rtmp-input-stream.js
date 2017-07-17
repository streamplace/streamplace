import ffmpeg from "fluent-ffmpeg";
import socketServerStream from "./socket-server-stream";
import mpegMungerStream from "./mpeg-munger-stream";
import debug from "debug";

const log = debug("sp:rtmp-input-stream");

export default function({ rtmpUrl }) {
  const socketServer = new socketServerStream();
  const mpegMunger = new mpegMungerStream();
  let logCounter = 0;
  const instance = ffmpeg()
    .on("error", (err, stdout, stderr) => {
      log("ffmpeg error", { err: err.toString(), stdout, stderr });
    })
    .on("codecData", data => {
      log("ffmpeg codecData", data);
    })
    .on("end", () => {
      log("ffmpeg end");
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
    })
    .input(rtmpUrl)
    .inputFormat("flv")
    .outputOptions(["-bsf:v h264_mp4toannexb", "-copyts", "-start_at_zero"])
    .videoCodec("copy")
    .audioCodec("copy")
    .outputFormat("mpegts")
    // Video out
    .output(`unix://${socketServer.path}`)
    .run();

  socketServer.pipe(mpegMunger);
  return mpegMunger;
}
