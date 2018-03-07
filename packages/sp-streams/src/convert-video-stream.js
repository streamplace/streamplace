import { socketEgressStream, socketIngressStream } from "sp-streams";
import debug from "debug";
import ffmpeg from "sp-streams/dist/ffmpeg";
import { PassThrough } from "stream";

const log = debug("sp:rtmp-output-stream");

/**
 * { rtmpUrl }
 */
export default function({ codec } = {}) {
  const passThrough = new PassThrough();
  passThrough.pipe(socketEgress);
  const socketEgress = new socketEgressStream();
  const socketIngress = new socketIngressStream();
  passThrough.pipe = socketIngress.pipe.bind(socketIngress);
  const instance = ffmpeg()
    .input(`unix://${socketEgress.path}`)
    .inputOptions(["-probesize 60000000", "-analyzeduration 10000000"])
    .inputFormat("webm")
    .videoCodec("copy")
    .audioCodec("aac")
    .outputFormat("flv")
    // Video out
    .output(`unix://${socketIngress.path}`);

  Promise.resolve()
    .then(() => {
      return socketEgress.getPath();
    })
    .then(() => {
      return socketIngress.getPath();
    })
    .then(() => {
      instance.run();
    });

  return socketEgress;
}
