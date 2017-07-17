import socketIngressStream from "../src/socket-ingress-stream";
import socketEgressStream from "../src/socket-egress-stream";
import colorbarsStream from "../src/colorbars-stream";
import ffmpeg, { killall } from "../src/ffmpeg";

describe("fast mpegts", () => {
  afterEach(() => {
    killall();
  });

  it("should import stuff", () => {
    expect(socketIngressStream).toBeDefined();
  });

  it(
    "should do things in a reasonable time",
    done => {
      let start;
      const colorbars = colorbarsStream();
      colorbars.once("data", () => {
        start = Date.now();
      });
      const socketEgress = socketEgressStream();
      const socketIngress = socketIngressStream();
      colorbars.pipe(socketEgress);
      const myFfmpeg = ffmpeg()
        .input(`unix://${socketEgress.path}`)
        .inputFormat("mpegts")
        .videoCodec("copy")
        .audioCodec("copy")
        .output(`unix://${socketIngress.path}`)
        .outputFormat("mpegts")
        .run();
      socketIngress.once("data", () => {
        /* eslint-disable no-console */
        console.log(`Recognizing stream took ${Date.now() - start}ms`);
        done();
      });
    },
    60000
  );
});
