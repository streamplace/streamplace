import socketEgressStream from "./socket-egress-stream";
import debug from "debug";
import ffmpeg from "./ffmpeg";

const log = debug("sp:rtmp-output-stream");

export default function({ rtmpUrl }) {
  const socketEgress = new socketEgressStream();
  const instance = ffmpeg()
    .input(`unix://${socketEgress.path}`)
    .inputOptions(["-probesize 60000000", "-analyzeduration 10000000"])
    .inputFormat("mpegts")
    .videoCodec("copy")
    .audioCodec("aac")
    .outputFormat("flv")
    // Video out
    .output(rtmpUrl);

  socketEgress.getPath().then(() => {
    instance.run();
  });

  return socketEgress;
}
