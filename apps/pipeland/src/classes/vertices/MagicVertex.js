
import winston from "winston";
import zmq from "zmq";
import _ from "underscore";

import InputVertex from "./InputVertex";
import SK from "../../sk";
import m from "../MagicFilters";

const MAIN_SWITCHER_LABEL = "mainSwitcher";
const SPLIT_SCREEN_TOP_SWITCHER_LABEL = "splitScreenTopSwitcher";
const SPLIT_SCREEN_BOTTOM_SWITCHER_LABEL = "splitScreenBottomSwitcher";
const SPLIT_SCREEN_BOTTOM_CROP_LABEL = "splitScreenBottomCrop";
const SPLIT_SCREEN_BOTTOM_OVERLAY_LABEL = "splitScreenBottomOverlay";

// This is hacky as hell but it doesn't seem to work if I send all messages at once.
const ZMQ_SEND_INTERVAL = 1000;

export default class MagicVertex extends InputVertex {
  constructor({id}) {
    super({id});
    this.rewriteStream = false;
    // this.debug = true;
    this.zmqQueue = [];
    this.zmqIsRunning = false;
    this.videoOutputURL = this.getUDPOutput();
    this.audioOutputURL = this.getUDPOutput();
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
    this.currentPositions = this.doc.params.positions;
    this.positionVertexHandle = SK.vertices.watch({id: this.id}).on("updated", ([vertex]) => {
      const newPositions = vertex.params.positions;
      Object.keys(vertex.params.positions).forEach((inputName) => {
        if (!_(this.currentPositions[inputName]).isEqual(newPositions[inputName])) {
          this.currentPositions[inputName] = newPositions[inputName];
          this.doZMQUpdate(inputName);
        }
      });
      this.currentPositions = newPositions;
    });
    this.doc.inputs.forEach((input) => {
      input.sockets.forEach((socket) => {
        socket.url = this.getUDPInput();
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

      this.doc.inputs.forEach((input, inputIdx) => {
        input.sockets.forEach((socket, socketIdx) => {
          socket.name = `${input.name}-${socketIdx}`;
          socket.inputName = input.name;
          this.ffmpeg
            .input(socket.url)
            .inputFormat("mpegts")
            .inputOptions([
              "-thread_queue_size 16384",
              // "-avioflags direct",
            ]);

          // Set up video input
          if (socket.type === "video") {
            videoInputSockets.push(socket);
            if (input.name === "background") {
              this.ffmpeg.magic(
                `${currentIdx}:v`,
                m.framerate("30"),
                m.scale(1920, 1080),
                `${socket.name}`
              );
            }
            else {
              const pos = this.doc.params.positions[input.name];
              this.ffmpeg.magic(
                `${currentIdx}:v`,
                m.framerate("30"),
                m.scale([pos.width, pos.height], {_label: `${input.name}-scale`}),
                `${socket.name}`
              );
            }
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

      // Do a series of overlays for the input
      let currentOverlayBG = "background-0";
      videoInputSockets.forEach((socket, i) => {
        if (socket.name === currentOverlayBG) {
          // First one doesn't need to overlay onto nothing. Return.
          return;
        }
        const newOverlayBG = `${socket.name}-overlay`;
        const pos = this.doc.params.positions[socket.inputName];
        this.ffmpeg.magic(
          currentOverlayBG,
          socket.name,
          m.overlay({
            x: pos.x,
            y: pos.y,
            _label: `${socket.inputName}-overlay`
          }),
          newOverlayBG
        );
        currentOverlayBG = newOverlayBG;
      });


      this.ffmpeg
        .outputOptions([
          "-copyts",
          "-copytb 1",
          "-vsync passthrough",
          // "-sws_flags +neighbor",
          "-probesize 2147483647",
          "-pix_fmt yuv420p",
          // "-profile:v baseline",
          // "-use_wallclock_as_timestamps 1",
          "-fflags +igndts",
          "-loglevel verbose",
        ])
        .magic(
          currentOverlayBG,
          m.zmq({bind_address: this.zmqAddress}),
          m.framerate("30"),
          "videoOutput"
        )
        .outputOptions([
          "-map [videoOutput]",
          "-b:v 4000k",
          "-allow_skip_frames 1",
          // "-preset veryfast",
          // "-x264opts keyint=60",
          // "-b:v 4000k",
          // "-minrate 4000k",
          // "-maxrate 4000k",
          // "-bufsize 1835k",
          // "-frame_drop_threshold 60",
        ])
        .once("progress", () => {
          const socket = zmq.socket("req");
          socket.on("connect", (fd, ep) => {
            this.zmqSocket = socket;
            if (!this.zmqIsRunning) {
              this._sendNextZMQMessage();
            }
            let idx = 0;
            let label = this.ffmpeg.filterLabels[MAIN_SWITCHER_LABEL];
          });
          socket.on("connect", (fd, ep) => {this.info("connect, endpoint:", ep);});
          socket.on("connect_delay", (fd, ep) => {this.info("connect_delay, endpoint:", ep);});
          socket.on("connect_retry", (fd, ep) => {this.info("connect_retry, endpoint:", ep);});
          socket.on("listen", (fd, ep) => {this.info("listen, endpoint:", ep);});
          socket.on("bind_error", (fd, ep) => {this.info("bind_error, endpoint:", ep);});
          socket.on("accept", (fd, ep) => {this.info("accept, endpoint:", ep);});
          socket.on("accept_error", (fd, ep) => {this.info("accept_error, endpoint:", ep);});
          socket.on("close", (fd, ep) => {this.info("close, endpoint:", ep);});
          socket.on("close_error", (fd, ep) => {this.info("close_error, endpoint:", ep);});
          socket.on("disconnect", (fd, ep) => {this.info("disconnect, endpoint:", ep);});
          socket.on("message", (msg) => {this.info("message: ", msg.toString());});
          socket.monitor(500, 0);
          socket.connect(`tcp://127.0.0.1:${this.zmqPort}`);
        })
        .output(this.videoOutputURL)
        .outputFormat("mpegts")
        .videoCodec("libopenh264")

        .magic(
          ...audioInputSockets.map(s => `${s.name}-adjusted`),
          m.amix({
            inputs: audioInputSockets.length
          }),
          "audioOutput"
        )

        .output(this.audioOutputURL)
        .outputOptions([
          "-map [audioOutput]"
        ])
        .outputFormat("mpegts")
        .audioCodec("aac");


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
