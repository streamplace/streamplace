
import winston from "winston";
import zmq from "zmq";
import _ from "underscore";

import InputVertex from "./InputVertex";
import SK from "../../sk";
import m from "../MagicFilters";

const MAIN_SWITCHER_LABEL = "mainSwitcher";
const SPLIT_SCREEN_TOP_SWITCHER_LABEL = "splitScreenTopSwitcher";
const SPLIT_SCREEN_BOTTOM_SWITCHER_LABEL = "splitScreenBottomSwitcher";

export default class MagicVertex extends InputVertex {
  constructor({id}) {
    super({id});
    this.rewriteStream = false;
    this.videoOutputURL = this.getUDPOutput();
    this.audioOutputURL = this.getUDPOutput();
  }

  handleInitialPull() {
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

  init() {
    super.init();
    try {
      this.ffmpeg = this.createffmpeg();
      this.zmqAddress = `tcp://0.0.0.0:${this.getTCP()}`;

      const videoInputSockets = [];
      const audioInputSockets = [];
      let currentIdx = 0;

      this.doc.inputs.forEach((input, inputIdx) => {
        input.sockets.forEach((socket, socketIdx) => {
          socket.name = `${input.name}-${socketIdx}`;
          this.ffmpeg
            .input(socket.url)
            .inputFormat("mpegts")
            .inputOptions([
              "-thread_queue_size 512",
              // "-avioflags direct",
            ]);

          // Set up video input
          if (socket.type === "video") {
            videoInputSockets.push(socket);
            this.ffmpeg.magic(
              `${currentIdx}:v`,
              m.framerate("30"),
              // m.realtime({limit: 2000000}),
              // m.setpts(`(RTCTIME - ${this.SERVER_START_TIME}) / (TB * 1000000)`),
              // m.setpts(`PTS-STARTPTS`),
              m.scale(1920, 1080),
              m.split(3),
              `${socket.name}-default`,
              `${socket.name}-splitTop`,
              `${socket.name}-splitBottom`
            );
          }

          // Set up audio input
          else if (socket.type === "audio") {
            audioInputSockets.push(socket);
            this.ffmpeg.magic(
              `${currentIdx}:a`,
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

      // Define splitscreen switcher
      this.ffmpeg
        .magic(
          ...videoInputSockets.map(s => `${s.name}-splitTop`),
          m.streamselect({
            inputs: videoInputSockets.length,
            map: 0,
            _label: SPLIT_SCREEN_TOP_SWITCHER_LABEL
          }),
          m.crop({
            w: "iw",
            h: "540",
            x: "0",
            y: "270",
          }),
          "splitScreenTop"
        )

        .magic(
          ...videoInputSockets.map(s => `${s.name}-splitBottom`),
          m.streamselect({
            inputs: videoInputSockets.length,
            map: 1,
            _label: SPLIT_SCREEN_BOTTOM_SWITCHER_LABEL
          }),
          m.crop({
            w: "iw",
            h: "540",
            x: "0",
            y: "270",
          }),
          "splitScreenBottom"
        )

        .magic(
          "splitScreenTop",
          "splitScreenBottom",
          m.vstack(),
          "splitScreenOut"
        );

      this.ffmpeg
        .outputOptions([
          "-copyts",
          "-copytb 1",
          "-async 1",
          "-vsync passthrough",
          "-probesize 2147483647",
          "-pix_fmt yuv420p",
          // "-profile:v baseline",
          // "-use_wallclock_as_timestamps 1",
          "-fflags +igndts",
          // "-loglevel verbose",
        ])
        .magic(
          ...videoInputSockets.map(s => `${s.name}-default`),
          "splitScreenOut",
          m.streamselect({inputs: videoInputSockets.length + 1, map: 2, _label: MAIN_SWITCHER_LABEL}),
          m.zmq({bind_address: this.zmqAddress}),
          m.framerate("30"),
          "videoOutput"
        )
        .outputOptions([
          "-map [videoOutput]",
          "-preset veryfast",
          // "-b:v 4000k",
          // "-minrate 4000k",
          // "-maxrate 4000k",
          // "-bufsize 1835k",
          // "-frame_drop_threshold 60",
        ])
        .once("progress", () => {
          const socket = zmq.socket("req");
          socket.on("connect", (fd, ep) => {
            let idx = 0;
            let label = this.ffmpeg.filterLabels[MAIN_SWITCHER_LABEL];
          //   setInterval(function() {
          //     idx += 1;
          //     if (idx >= videoInputNames.length + 1) {
          //       idx = 0;
          //     }
          //     socket.send(`${label} map ${idx}`);
          //   }, 3000);
          //   this.info("connect, endpoint:", ep);
          });
          socket.on("connect_delay", (fd, ep) => {this.info("connect_delay, endpoint:", ep);});
          socket.on("connect_retry", (fd, ep) => {this.info("connect_retry, endpoint:", ep);});
          socket.on("listen", (fd, ep) => {this.info("listen, endpoint:", ep);});
          socket.on("bind_error", (fd, ep) => {this.info("bind_error, endpoint:", ep);});
          socket.on("accept", (fd, ep) => {this.info("accept, endpoint:", ep);});
          socket.on("accept_error", (fd, ep) => {this.info("accept_error, endpoint:", ep);});
          socket.on("close", (fd, ep) => {this.info("close, endpoint:", ep);});
          socket.on("close_error", (fd, ep) => {this.info("close_error, endpoint:", ep);});
          socket.on("disconnect", (fd, ep) => {this.info("disconnect, endpoint:", ep);});
          socket.monitor(500, 0);
          socket.connect(this.zmqAddress);
        })
        .output(this.videoOutputURL)
        .outputFormat("mpegts")
        .videoCodec("libx264")

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
