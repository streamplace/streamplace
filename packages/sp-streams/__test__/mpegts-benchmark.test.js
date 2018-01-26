import socketIngressStream from "../src/socket-ingress-stream";
import socketEgressStream from "../src/socket-egress-stream";
import colorbarsStream from "../src/colorbars-stream";
import ffmpeg, { killall } from "../src/ffmpeg";

// This file tries to get the mpegts produced by ffmpeg to be recognized by
// ffmpeg as fast as possible.
const tests = {
  "regular vanilla with nothing": proc => {
    // Nothing. There's nothing here.
  },
  "-mpegts_flags pat_pmt_at_frames": proc => {
    proc.outputOptions("-mpegts_flags pat_pmt_at_frames");
  },
  "-muxrate 50": proc => {
    proc.outputOptions("-muxrate 50");
  },
  "-mpegts_m2ts_mode 1": proc => {
    proc.outputOptions("-mpegts_m2ts_mode 1");
    // mpegMunger can't handle this apparently. So let's verify it keeps
    // breaking, so we know if that changes.
    return false;
  },
  "-probesize 500000": proc => {
    proc.outputOptions("-probesize 500000");
  },
  "-probesize 50000": proc => {
    proc.outputOptions("-probesize 50000");
  },
  "-analyzeduration 50000": proc => {
    proc.outputOptions("-analyzeduration 50000");
  },
  "-fflags nobuffer": proc => {
    proc.outputOptions("-fflags nobuffer");
  },
  "-max_streams 2": proc => {
    proc.outputOptions("-max_streams 2");
  },
  "-resync_size 1024": proc => {
    proc.outputOptions("-resync_size 1024");
  }
};

xdescribe("fast mpegts", () => {
  afterEach(() => {
    killall();
  });

  it("should import stuff", () => {
    expect(socketIngressStream).toBeDefined();
  });

  Object.keys(tests).forEach(description => {
    const testFn = tests[description];

    it(
      `mpegts benchmark: ${description}`,
      done => {
        let start;
        const colorbars = colorbarsStream({ start: false });
        const shouldFail = testFn(colorbars.ffmpeg) === false;
        colorbars.once("data", () => {
          start = Date.now();
        });
        colorbars.ffmpeg.run();
        const socketEgress = socketEgressStream();
        const socketIngress = socketIngressStream();
        setTimeout(() => {
          colorbars.pipe(socketEgress);
        }, 2000);
        let finished = false;
        const myFfmpeg = ffmpeg()
          .input(`unix://${socketEgress.path}`)
          .inputFormat("mpegts")
          .videoCodec("copy")
          .audioCodec("copy")
          .output(`unix://${socketIngress.path}`)
          .outputFormat("mpegts")
          .run();
        /* eslint-disable no-console */
        socketIngress.once("data", () => {
          finished = true;
          console.log(`${description}: ${Date.now() - start}ms`);
          if (shouldFail) {
            throw new Error("Expected failure!");
          }
          done();
        });
        colorbars.on("end", () => {
          if (finished) {
            return;
          }
          if (!shouldFail) {
            return done(new Error("Colorbars stream ended."));
          }
          console.log(`${description}: errored`);
          done();
        });
      },
      15000
    );
  });
});
