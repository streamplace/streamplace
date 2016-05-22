
import winston from "winston";
import zmq from "zmq";
import _ from "underscore";

import InputVertex from "./InputVertex";
import SK from "../../sk";
import m from "../MagicFilters";

// This is hacky as hell but it doesn't seem to work if I send all messages at once.
const ZMQ_SEND_INTERVAL = 1000;

export default class CompositeVertex extends InputVertex {
  constructor({id}) {
    super({id});
    this.streamFilters = [];
    // this.debug = true;
    this.zmqQueue = [];
    this.zmqIsRunning = false;
    this.videoOutputURL = this.transport.getOutputURL();
    this.audioOutputURL = this.transport.getOutputURL();
  }

  cleanup() {
    super.cleanup();
    this.positionVertexHandle.stop();

    if (this.zmqSocket) {
      try {
        this.zmqSocket.unmonitor();
        this.zmqSocket.disconnect(this.zmqAddress);
      }
      catch (e) {
        this.error("Error disconnecting from ZMQ: ", e.stack);
      }
    }
  }

  handleInitialPull() {
    this.sceneHandle = SK.scenes.watch({broadcastId: this.doc.broadcastId})
    .on("data", (scenes) => {
      this.scenes = scenes.sort(function(a, b) {
        return a.id > b.id;
      });
    })
    .catch(::this.error);

    this.doc.inputs.forEach((input) => {
      input.sockets.forEach((socket) => {
        socket.url = this.transport.getInputURL();
      });
    });
    const newVertex = {
      inputs: this.doc.inputs,
      outputs: [{
        name: "default",
        sockets: [{
          url: this.videoOutputURL,
          type: "video",
        }, {
          url: this.audioOutputURL,
          type: "audio",
        }]
      }]

    };
    SK.vertices.update(this.doc.id, newVertex)
    .then(() => {
      return this.sceneHandle;
    })
    .then((scenes) => {
      this.init();
    })
    .catch((err) => {
      winston.error(err);
    });
  }

  _sendNextZMQMessage() {
    // If there's nothing to do, we're not running anymore.
    if (this.zmqQueue.length === 0) {
      this.zmqIsRunning = false;
      return;
    }
    // If we're not connected yet, chill. We'll get called again when we do.
    if (!this.zmqSocket) {
      this.zmqIsRunning = false;
      return;
    }
    // Otherwise, send a message and queue the next one.
    const msg = this.zmqQueue.pop();
    this.info(`ZMQ: ${msg}`);
    this.zmqSocket.send(msg);
    this.zmqIsRunning = true;
    setTimeout(this._sendNextZMQMessage.bind(this), ZMQ_SEND_INTERVAL);
  }

  doZMQUpdate(inputName) {
    const send = (msg) => {
      this.zmqQueue.push(msg);
    };
    const pos = this.currentPositions[inputName];
    if (!pos) {
      return;
    }
    const overlayLabel = this.ffmpeg.filterLabels[`${inputName}-overlay`];
    const scaleLabel = this.ffmpeg.filterLabels[`${inputName}-scale`];
    send(`${overlayLabel} x ${pos.x}`);
    send(`${overlayLabel} y ${pos.y}`);
    send(`${scaleLabel} width ${pos.width}`);
    send(`${scaleLabel} height ${pos.height}`);
    if (!this.zmqIsRunning) {
      this._sendNextZMQMessage();
    }
  }

  init() {
    super.init();
    try {
      this.ffmpeg = this.createffmpeg();
      this.zmqPort = this.getTCP();
      this.zmqAddress = `tcp://*:${this.zmqPort}`;

      const videoInputSockets = [];
      const audioInputSockets = [];
      let currentIdx = 0;

      const sceneBackgroundIds = this.scenes.map((scene) => {
        return `bg_${scene.id}`;
      });

      this.doc.inputs.forEach((input, inputIdx) => {
        input.sockets.forEach((socket, socketIdx) => {
          socket.name = `${input.name}-${socketIdx}`;
          socket.inputName = input.name;
          this.ffmpeg
            .input(socket.url)
            .inputFormat("mpegts")
            .inputOptions([
              "-analyzeduration 10000000",
              "-noaccurate_seek", // Not really sure if this helps, but we certainly don't need
                                  // any kind of accurate seek.
              "-probesize 60000000",
              "-thread_queue_size 16384",
              // "-avioflags direct",
            ]);

          // Set up video input
          if (socket.type === "video") {
            const regionsForInput = [];
            this.scenes.forEach((scene) => {
              scene.regions.forEach((region, i) => {
                if (region.inputId === input.name) {
                  regionsForInput.push(`${scene.id}_${i}`);
                }
              });
            });
            videoInputSockets.push(socket);

            this.ffmpeg.magic(
              `${currentIdx}:v`,
              m.framerate("30"),
              m.split(regionsForInput.length),
              ...regionsForInput
            );
          }

          // Set up audio input
          else if (socket.type === "audio") {
            audioInputSockets.push(socket);
            this.ffmpeg.magic(
              `${currentIdx}:a`,
              m.aresample({
                async: 1,
                min_hard_comp: 0.100000,
                first_pts: 0
              }),
              m.volume({
                _label: `${socket.name}-volume`
              }),
              `${socket.name}-adjusted`
            );
          }

          else {
            throw new Error(`Unknown input type: ${input.type}`);
          }
          currentIdx += 1;
        });
      });

      currentIdx += 1;

      let scenesForSelector = [];

      this.scenes.forEach((scene) => {
        const [firstRegion, ...otherRegions] = scene.regions;
        let currentInput = `${scene.id}_0`;
        this.ffmpeg.magic(
          currentInput,
          m.scale({
            w: firstRegion.width,
            h: firstRegion.height
          }),
          m.pad({
            width: 1920,
            height: 1080,
            x: firstRegion.x,
            y: firstRegion.y,
          }),
          `${scene.id}_0_padded`
        );
        currentInput = `${scene.id}_0_padded`;

        otherRegions.forEach((region, i) => {
          i = i + 1; // We already did the first one
          const thisInput = `${scene.id}_${i}`;
          const newOutput = `${thisInput}_overlay`;
          this.ffmpeg.magic(
            thisInput,
            m.scale({
              w: region.width,
              h: region.height,
            }),
            `${thisInput}_scaled`
          );
          this.ffmpeg.magic(
            currentInput,
            `${thisInput}_scaled`,
            m.overlay({
              x: region.x,
              y: region.y,
            }),
            newOutput
          );
          currentInput = newOutput;
        });
        scenesForSelector.push(currentInput);
      });

      if (scenesForSelector.length > 1) {
        this.ffmpeg.magic(
          ...scenesForSelector,
          m.streamselect({inputs: scenesForSelector.length, map: 0}),
          m.framerate("30"),
          "videoOutput"
        );
      }
      else {
        this.ffmpeg.magic(
          ...scenesForSelector,
          m.framerate("30"),
          "videoOutput"
        );
      }

      this.ffmpeg.magic(
        ...audioInputSockets.map(s => `${s.name}-adjusted`),
        m.amix({
          inputs: audioInputSockets.length
        }),
        "audioOutput"
      );

      this.ffmpeg.output(this.audioOutputURL)
        .outputOptions([
          "-map [audioOutput]"
        ])
        .outputFormat("mpegts")
        .audioCodec("aac");

      this.ffmpeg
        .output(this.videoOutputURL)
        .outputOptions([
          "-map [videoOutput]",
        ])
        .outputFormat("mpegts")
        .videoCodec("libx264");

      this.ffmpeg
        .outputOptions([
          "-copyts",
          "-copytb 1",
          "-vsync passthrough",

          // I think this flag just causes it to discard the analysis buffer when the stream starts
          "-fflags +nobuffer",

          // "-sws_flags +neighbor",
          "-pix_fmt yuv420p",
          // "-profile:v baseline",
          // "-use_wallclock_as_timestamps 1",
          "-fflags +igndts",
          "-loglevel verbose",
        ])
        // .magic(
        //   currentOverlayBG,
        //   m.zmq({bind_address: this.zmqAddress}),
        //   m.framerate("30"),
        //   "videoOutput"
        // )
        .outputOptions([
          "-b:v 4000k",
          "-allow_skip_frames 1",
          "-preset veryfast",
          "-x264opts keyint=60",
          "-b:v 4000k",
          // "-minrate 4000k",
          "-maxrate 4000k",
          // "-bufsize 1835k",
          // "-frame_drop_threshold 60",
        ]);
        // .once("progress", () => {
        //   const socket = zmq.socket("req");
        //   socket.on("connect", (fd, ep) => {
        //     this.zmqSocket = socket;
        //     if (!this.zmqIsRunning) {
        //       this._sendNextZMQMessage();
        //     }
        //     // Unfortunately if the filtergraph re-inits, our ZMQ changes don't get preserved. So
        //     // as soon as ZMQ boots back up, go ahead and inform them of our changes again.
        //     this.doc.inputs.forEach((input) => {
        //       this.doZMQUpdate(input.name);
        //     });
        //     let idx = 0;
        //     // let label = this.ffmpeg.filterLabels[MAIN_SWITCHER_LABEL];
        //   });
        //   socket.on("connect", (fd, ep) => {this.info("connect, endpoint:", ep);});
        //   socket.on("connect_delay", (fd, ep) => {this.info("connect_delay, endpoint:", ep);});
        //   socket.on("connect_retry", (fd, ep) => {this.info("connect_retry, endpoint:", ep);});
        //   socket.on("listen", (fd, ep) => {this.info("listen, endpoint:", ep);});
        //   socket.on("bind_error", (fd, ep) => {this.info("bind_error, endpoint:", ep);});
        //   socket.on("accept", (fd, ep) => {this.info("accept, endpoint:", ep);});
        //   socket.on("accept_error", (fd, ep) => {this.info("accept_error, endpoint:", ep);});
        //   socket.on("close", (fd, ep) => {this.info("close, endpoint:", ep);});
        //   socket.on("close_error", (fd, ep) => {this.info("close_error, endpoint:", ep);});
        //   socket.on("disconnect", (fd, ep) => {this.info("disconnect, endpoint:", ep);});
        //   socket.on("message", (msg) => {this.info("message: ", msg.toString());});
        //   socket.monitor(500, 0);
        //   socket.connect(`tcp://127.0.0.1:${this.zmqPort}`);
        // })



        // .input(this.inputURL)
        // .inputFormat("mpegts")
        // // .inputOptions("-itsoffset 00:00:05")
        // .outputOptions([
        // ])
        // .videoCodec("libx264")
        // .audioCodec("pcm_s16le")
        // .outputOptions([
        //   "-preset ultrafast",
        //   "-tune zerolatency",
        //   "-x264opts keyint=5:min-keyint=",
        //   "-pix_fmt yuv420p",
        //   "-filter_complex",
        //   [
        //     `[0:a]asetpts='(RTCTIME - ${this.SERVER_START_TIME}) / (TB * 1000000)'[out_audio]`,
        //     `[0:v]setpts='(RTCTIME - ${this.SERVER_START_TIME}) / (TB * 1000000)'[out_video]`,
        //   ].join(";")
        // ])

        // // Video output
        // .output(this.videoOutputURL)
        // .outputOptions([
        //   "-map [out_video]",
        // ])
        // .outputFormat("mpegts")

        // // Audio output
        // .output(this.audioOutputURL)
        // .outputOptions([
        //   "-map [out_audio]",
        // ])
        // .outputFormat("mpegts");

      this.ffmpeg.run();
    }
    catch (err) {
      this.error(err);
      this.retry();
    }
  }
}
