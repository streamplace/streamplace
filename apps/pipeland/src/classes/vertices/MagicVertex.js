
import winston from "winston";
import zmq from "zmq";
import _ from "underscore";

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
    this.videoOutputURL = this.getUDPOutput();
    this.audioOutputURL = this.getUDPOutput();
  }

  handleInitialPull() {
    Object.keys(this.doc.inputs).forEach((inputName) => {
      const input = this.doc.inputs[inputName];
      input.socket = this.getUDPInput();
    });
    const newVertex = {
      inputs: this.doc.inputs,
      outputs: {
        video: {
          socket: this.videoOutputURL,
          type: "video",
        },
        audio: {
          socket: this.audioOutputURL,
          type: "audio",
        },
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
      this.zmqAddress = `tcp://0.0.0.0:${this.getTCP()}`;
      const videoInputNames = [];
      const audioInputNames = [];
      _(this.doc.inputs).each((details, name) => {
        if (details.type === "video") {
          videoInputNames.push(name);
        }
        else if (details.type === "audio") {
          audioInputNames.push(name);
        }
      });

      // Set up video input stuff
      videoInputNames.forEach((inputName, i) => {
        const input = this.doc.inputs[inputName];
        this.ffmpeg
          .input(input.socket)
          .inputFormat("mpegts")
          .magic(
            `${i}:v`,
            m.framerate("30"),
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

      const audioInputAdjustedNames = audioInputNames.map((inputName, i) => {
        const input = this.doc.inputs[inputName];
        const myIndex = i + videoInputNames.length; // wow that's ugly
        const muxedName = `audio${myIndex}volume`;
        this.ffmpeg
          .input(input.socket)
          .inputFormat("mpegts")
          .magic(
            `${myIndex}:a`,
            m.volume({
              _label: `audio${i}volume`,
            }),
            muxedName
          );
        return muxedName;
      });

      // Define splitscreen switcher
      this.ffmpeg
        .magic(
          ...videoInputNames.map(name => name + "splitTop"),
          m.streamselect({
            inputs: videoInputNames.length,
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
          ...videoInputNames.map(name => name + "splitBottom"),
          m.streamselect({
            inputs: videoInputNames.length,
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
          "-vsync passthrough",
          "-probesize 2147483647",
          "-pix_fmt yuv420p",
          // "-profile:v baseline",
          // "-use_wallclock_as_timestamps 1",
          "-fflags +igndts",
          // "-loglevel verbose",
        ])
        .magic(
          ...videoInputNames.map(name => name + "default"),
          "splitScreenOut",
          m.streamselect({inputs: videoInputNames.length + 1, map: 2, _label: MAIN_SWITCHER_LABEL}),
          m.zmq({bind_address: this.zmqAddress}),
          m.framerate("30"),
          "videoOutput"
        )
        .outputOptions([
          "-map [videoOutput]",
          "-preset medium",
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
          ...audioInputAdjustedNames,
          m.amix({
            inputs: audioInputAdjustedNames.length
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
