import config from "sp-configuration";
import { SPClient } from "sp-client";
import fluent from "fluent-ffmpeg";
import { PassThrough } from "stream";

const DOMAIN = config.require("DOMAIN");

describe("conformance", () => {
  let SP;
  let procs;

  const sendDemoStream = streamKey => {
    return new Promise((resolve, reject) => {
      const proc = fluent()
        .input("testsrc=size=1920x1080:rate=30:duration=-1")
        .inputFormat("lavfi")
        .inputOptions(["-re"])
        .input("sine=frequency=1000:duration=99999999999999999")
        .inputFormat("lavfi")
        .videoCodec("libx264")
        .audioCodec("aac")
        .outputOptions(["-preset veryfast"])
        .outputFormat("flv")
        .output(`rtmp://${DOMAIN}/stream/${streamKey}`)
        // .on("start", function(data) {
        //   console.log(data);
        // })
        .once("progress", function() {
          resolve(proc);
        })
        .once("error", function(err) {
          if (!err.message.includes("SIGKILL")) {
            reject(err);
            throw err;
          }
        });
      proc.run();
      procs.push(proc);
    });
  };

  const getFromRtmp = streamKey => {
    return new Promise((resolve, reject) => {
      let proc;
      const newProc = () => {
        const passThrough = new PassThrough();
        proc = fluent()
          .input(`rtmp://${DOMAIN}/stream/${streamKey}`)
          .inputFormat("flv")
          .videoCodec("copy")
          .audioCodec("copy")
          .outputFormat("mpegts")
          .once("error", function(err) {
            if (!err.message.includes("SIGKILL")) {
              clearInterval(interval);
              reject(err);
              throw err;
            }
          })
          .output(passThrough);
        proc.run();
        passThrough.resume();
        passThrough.once("data", () => {
          clearInterval(interval);
          resolve();
        });
        procs.push(proc);
      };
      newProc();
      let interval = setInterval(newProc, 5000);
    });
  };

  beforeEach(() => {
    procs = [];
    SP = new SPClient();
    return SP.connect();
  });
  afterEach(() => {
    procs.forEach(proc => {
      try {
        proc.kill();
      } catch (e) {
        ("it's okay, it was probably already dead");
      }
    });
    return Promise.all(
      ["inputs", "outputs", "broadcasts"].map(name => {
        return SP[name].find({ userId: SP.user.id }).then(resources => {
          return Promise.all(
            resources.map(resource => {
              return SP[name].delete(resource.id);
            })
          );
        });
      })
    ).then(() => {
      return SP.disconnect();
    });
  });
  it(
    "should be able to stream to itself",
    () => {
      let input1;
      let input2;
      let broadcast;
      return SP.inputs
        .create({})
        .then(input => {
          input1 = input;
          expect(input).toBeTruthy();
          expect(typeof input.id).toBe("string");
          return sendDemoStream(input.streamKey);
        })
        .then(proc => {
          return SP.broadcasts.create({
            active: true,
            sources: [
              {
                kind: "Input",
                id: input1.id
              }
            ]
          });
        })
        .then()
        .then(_broadcast => {
          broadcast = _broadcast;
          return SP.inputs.create({});
        })
        .then(input => {
          input2 = input;
          return SP.outputs.create({
            broadcastId: broadcast.id,
            url: `rtmp://dev-sp-rtmp-server.default.svc.cluster.local/stream/${input2.streamKey}`
          });
        })
        .then(output => {
          return getFromRtmp(input2.streamKey);
        });
    },
    90000
  );
});
