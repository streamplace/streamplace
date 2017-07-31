import socketIngressStream from "./socket-ingress-stream";
import mpegMungerStream from "./mpeg-munger-stream";
import debug from "debug";
import ffmpeg from "./ffmpeg";

const log = debug("sp:rtmp-input-stream");

export default function({ rtmpUrl }) {
  const socketServer = new socketIngressStream();
  const mpegMunger = new mpegMungerStream();
  const instance = ffmpeg()
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
