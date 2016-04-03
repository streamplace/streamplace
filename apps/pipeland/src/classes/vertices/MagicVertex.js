
import winston from "winston";

import BaseVertex from "./BaseVertex";
import SK from "../../sk";
import m from "../MagicFilters";

export default class MagicVertex extends BaseVertex {
  constructor({id}) {
    super({id});
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
            m.setpts(`(RTCTIME - ${this.SERVER_START_TIME}) / (TB * 1000000)`),
            m.scale(1920, 1080),
            inputName
          );
      });

      this.ffmpeg
        .outputOptions([
          "-copyts",
          "-vsync passthrough",
        ])
        .magic(
          ...inputNames,
          m.streamselect({inputs: inputNames.length, map: 0}),
          "output"
        )
        .outputOptions([
          "-map [output]",
          "-preset veryfast",
          "-r 30",
        ])
        .outputFormat("mpegts")
        .videoCodec("libx264")
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
