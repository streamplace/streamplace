import socketIngressStream from "./socket-ingress-stream";
import mpegMungerStream from "./mpeg-munger-stream";
import debug from "debug";
import ffmpeg from "./ffmpeg";

const log = debug("sp:rtmp-input-stream");

export default function({ rtmpUrl }) {
  const socketIngress = new socketIngressStream();
  const mpegMunger = new mpegMungerStream();
  const instance = ffmpeg()
    .input(rtmpUrl)
    .inputFormat("flv")
    .outputOptions(["-bsf:v h264_mp4toannexb", "-copyts", "-start_at_zero"])
    .videoCodec("copy")
    .audioCodec("copy")
    .outputFormat("mpegts")
    // Video out
    .output(`unix://${socketIngress.path}`);
  instance.run();

  socketIngress.pipe(mpegMunger);
  mpegMunger.notifyPTS = function(pts) {
    mpegMunger.currentPTS = pts;
  };
  mpegMunger.on("end", () => {
    instance.kill();
    socketIngress.destroy();
  });
  return mpegMunger;
}
