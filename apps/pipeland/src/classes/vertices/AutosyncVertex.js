
import MpegMungerStream from "mpeg-munger";
import winston from "winston";

import QuietStream from "../QuietStream";
import BaseVertex from "./BaseVertex";
import SK from "../../sk";
import ENV from "../../env";

const TIME_BASE = 90;

const PROFILE = {
  "checksum_scheme": "crc32",
  "inner_fec_scheme": "v27",
  "outer_fec_scheme": "none",
  "mod_scheme": "psk2",
  "frame_length": 25,
  "modulation": {
    "center_frequency": 4200,
    "gain": 0.15
  },
  "interpolation": {
    "samples_per_symbol": 10,
    "symbol_delay": 4,
    "excess_bandwidth": 0.35
  },
  "encoder_filters": {
    "dc_filter_alpha": 0.01
  },
  "resampler": {
    "delay": 13,
    "bandwidth": 0.45,
    "attenuation": 60,
    "filter_bank_size": 64
  }
};

export default class AutosyncVertex extends BaseVertex {
  constructor({id}) {
    super({id});
    // Incoming Stream --> mpeg-munger --> ffmpeg --> quiet-js
    this.audioInputURL = this.transport.getInputURL();
    this.streamToFFmpegURL = this.transport.getInputURL();
    this.streamFromFFmpegURL = this.transport.getOutputURL();
  }

  notifyPTS(pts) {
    this.lastPTS = pts;
  }

  handleInitialPull() {
    super.handleInitialPull();
    SK.vertices.update(this.doc.id, {
      inputs: [{
        name: "default",
        sockets: [{
          url: this.audioInputURL,
          type: "audio"
        }]
      }]
    })
    .then(() => {
      return SK.vertices.findOne(this.doc.params.inputVertexId);
    })
    .then((inputVertex) => {
      this.inputId = inputVertex.params.inputId;
      this.init();
    })
    .catch((err) => {
      this.error(err);
    });
  }

  _doNextSync() {
    SK.inputs.update(this.inputId, {nextSync: Date.now() + 5000})
    .catch((err) => {
      winston.error("Error setting sync time", err);
    });

    // Haven't gotten one in fifteen seconds? Boo! Try again...
    setTimeout(::this._doNextSync, 15000);
  }

  _onReceiveSync(buf) {
    const [time] = new Float64Array(buf.buffer, buf.byteOffset, 1);
    const startedAt = Math.round(time - (this.lastPTS / TIME_BASE));
    this.info(`We appear to have started at ${startedAt}`);
    SK.vertices.update(this.doc.params.inputVertexId, {
      syncStartTime: startedAt,
    })
    .then(() => {
      // Okay, we did our job. Bye!
      SK.vertices.delete(this.doc.id);
    })
    .catch(::this.error);
  }

  init() {
    const inputStream = new this.transport.InputStream({url: this.audioInputURL});

    const mpegStream = new MpegMungerStream();
    mpegStream.transformPTS = (pts) => {
      this.notifyPTS(pts);
      return pts;
    };
    inputStream.pipe(mpegStream);

    const streamToFFmpeg = new this.transport.OutputStream({url: this.streamToFFmpegURL});
    mpegStream.pipe(streamToFFmpeg);

    const streamFromFFmpeg = new this.transport.InputStream({url: this.streamFromFFmpegURL});

    const quietStream = new QuietStream({profile: PROFILE});
    streamFromFFmpeg.pipe(quietStream);

    // Once we're processing a WAV stream, we can go ahead and trigger a sync
    streamFromFFmpeg.once("data", () => {
      this._doNextSync();
    });

    quietStream.on("data", ::this._onReceiveSync);

    this.ffmpeg = this.createffmpeg()
      .input(this.streamToFFmpegURL)
      .inputFormat("mpegts")
      .inputOptions([
        "-thread_queue_size 512",
      ])
      .audioCodec("pcm_f32le")
      .output(this.streamFromFFmpegURL)
      .outputFormat("f32le")
      .outputOptions([
        "-ac 1"
      ])
      .run();
  }


  cleanup() {
    super.cleanup();
  }
}
