
import winston from "winston";

import BaseVertex from "./BaseVertex";
import SK from "../../sk";

export default class AudioMixVertex extends BaseVertex {
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

      Object.keys(this.doc.inputs).forEach((inputName) => {
        const input = this.doc.inputs[inputName];
        this.ffmpeg
          .input(input.socket)
          .inputFormat("mpegts");
      });

      this.ffmpeg
        .outputOptions([
          "-copyts",
          "-vsync passthrough",
          `-filter_complex amix=inputs=${this.doc.params.inputCount}`,
        ])
        .outputFormat("mpegts")
        .audioCodec("libmp3lame")
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
