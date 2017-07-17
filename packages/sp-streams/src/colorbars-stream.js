import ffmpeg from "./ffmpeg";
import { PassThrough } from "stream";
import socketIngressStream from "./socket-ingress-stream";

export function colorBarsFfmpeg() {
  return ffmpeg()
    .input("testsrc=size=1920x1080:rate=30")
    .inputOptions("-re")
    .inputFormat("lavfi")
    .input("sine=frequency=1000")
    .inputFormat("lavfi")
    .videoCodec("libx264")
    .audioCodec("aac");
}

export default function colorbarsStream() {
  const socketIngress = socketIngressStream();

  colorBarsFfmpeg()
    .output(`unix://${socketIngress.path}`)
    .outputFormat("mpegts")
    .run();

  return socketIngress;
}
