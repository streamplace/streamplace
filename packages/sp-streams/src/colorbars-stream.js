import ffmpeg from "./ffmpeg";
import { PassThrough } from "stream";
import socketIngressStream from "./socket-ingress-stream";
import mpegMungerStream from "./mpeg-munger-stream";

export function colorBarsFfmpeg() {}

export default function colorbarsStream({ start = true } = {}) {
  const socketIngress = socketIngressStream();
  const mpegMunger = mpegMungerStream();

  const myFfmpeg = ffmpeg()
    .input("testsrc=size=1920x1080:rate=30")
    .inputOptions("-re")
    .inputFormat("lavfi")
    .input("sine=frequency=1000")
    .inputFormat("lavfi")
    .videoCodec("libx264")
    .audioCodec("aac")
    .output(`unix://${socketIngress.path}`)
    .outputFormat("mpegts");

  if (start) {
    myFfmpeg.run();
  }

  socketIngress.pipe(mpegMunger);

  mpegMunger.ffmpeg = myFfmpeg;

  return mpegMunger;
}
