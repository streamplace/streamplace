
import winston from "winston";
import zmq from "zmq";

import BaseVertex from "./BaseVertex";
import SK from "../../sk";
import m from "../MagicFilters";

const MAIN_SWITCHER_LABEL = "mainSwitcher";
const SPLIT_SCREEN_TOP_SWITCHER_LABEL = "splitScreenTopSwitcher";
const SPLIT_SCREEN_BOTTOM_SWITCHER_LABEL = "splitScreenBottomSwitcher";

export default class MagicVertex extends BaseVertex {
  constructor({id}) {
    super({id});
    // this.debug = true;
    this.outputURL = this.getUDP();
  }

  handleInitialPull() {
    Object.keys(this.doc.inputs).forEach((inputName) => {
      const input = this.doc.inputs[inputName];
      input.socket = this.getUDPInput();
    });
    const newVertex = {
      inputs: this.doc.inputs,
      outputs: {
        default: {
          socket: this.outputURL,
        }
      }
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
    try {
      this.ffmpeg = this.createffmpeg();

      const inputNames = Object.keys(this.doc.inputs);

      inputNames.forEach((inputName, i) => {
        const input = this.doc.inputs[inputName];
        this.ffmpeg
          .input(input.socket)
          .inputFormat("mpegts")
          .magic(
            `${i}:v`,
            // m.realtime({limit: 2000000}),
            // m.setpts(`(RTCTIME - ${this.SERVER_START_TIME}) / (TB * 1000000)`),
            // m.setpts(`PTS-STARTPTS`),
            m.scale(1920, 1080),
            m.split(3),
            inputName + "default",
            inputName + "splitTop",
            inputName + "splitBottom"
          )
        .inputOptions([
          "-thread_queue_size 512",
          // "-avioflags direct",
        ]);
      });

      // Define splitscreen switcher
      this.ffmpeg
        .magic(
          ...inputNames.map(name => name + "splitTop"),
          m.streamselect({
            inputs: inputNames.length,
            map: 0,
            _label: SPLIT_SCREEN_TOP_SWITCHER_LABEL
          }),
          m.scale(1920, 540),
          "splitScreenTop"
        )

        .magic(
          ...inputNames.map(name => name + "splitBottom"),
          m.streamselect({
            inputs: inputNames.length,
            map: 1,
            _label: SPLIT_SCREEN_BOTTOM_SWITCHER_LABEL
          }),
          m.scale(1920, 540),
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
          "-vsync passthrough",
          "-probesize 2147483647",
          // "-use_wallclock_as_timestamps 1",
          "-fflags +igndts",
          // "-loglevel debug",
        ])
        .magic(
          ...inputNames.map(name => name + "default"),
          "splitScreenOut",
          m.streamselect({inputs: inputNames.length + 1, map: 2, _label: MAIN_SWITCHER_LABEL}),
          m.zmq({bind_address: "tcp://0.0.0.0:5555"}),
          "output"
        )
        .outputOptions([
          "-map [output]",
          "-preset veryfast",
          // "-frame_drop_threshold 60",
        ])
        .outputFormat("mpegts")
        .videoCodec("libx264")
        .once("progress", () => {
          const socket = zmq.socket("req");
          socket.on("connect", (fd, ep) => {
            let idx = 0;
            let label = this.ffmpeg.filterLabels[MAIN_SWITCHER_LABEL];
            setInterval(function() {
              idx += 1;
              if (idx >= inputNames.length + 1) {
                idx = 0;
              }
              socket.send(`${label} map ${idx}`);
            }, 3000);
            this.info("connect, endpoint:", ep);
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
          socket.connect("tcp://0.0.0.0:5555");
        })
        .output(this.outputURL);


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
