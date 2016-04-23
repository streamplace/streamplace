/**
 * This guy isn't really a full vertex like everything else. It's not backed by an API vertex
 * object -- instead it's run ad-hoc by nodes that need a video/audio feed for a "no signal" state.
 */

import path from "path";

import BaseVertex from "./BaseVertex";

export default class NoSignalVertex extends BaseVertex {
  constructor(params) {
    super(params);
    this.videoOutputURL = this.getUDPOutput();
    this.audioOutputURL = this.getUDPOutput();
    this.init();
    // this.debug = true;
  }

  // Old -- just copied from a file. Problems.
  // init() {
  //   try {
  //     this.ffmpeg = this.createffmpeg()
  //       .input(path.resolve(__dirname, "..", "..", "media", "nosignal.ts"))
  //       .inputFormat("mpegts")
  //       .inputOptions([
  //         "-re",
  //         "-stream_loop -1",
  //       ])

  //       // Video Output
  //       .output(this.videoOutputURL)
  //       .videoCodec("copy")
  //       .outputFormat("mpegts")
  //       .outputOptions([
  //         "-map 0:v",
  //       ])

  //       // Audio Output
  //       .output(this.audioOutputURL)
  //       .audioCodec("copy")
  //       .outputFormat("mpegts")
  //       .outputOptions([
  //         "-map 0:a",
  //       ])

  //     this.ffmpeg.run();
  //   }
  //   catch (err) {
  //     this.error(err);
  //     this.retry();
  //   }
  // }
  init() {
    try {
      this.ffmpeg = this.createffmpeg()
        .input("testsrc=size=1920x1080:rate=30")
        .inputFormat("lavfi")
        .inputOptions("-re")

        .input("sine=frequency=1000")
        .inputFormat("lavfi")
        .inputOptions("-re")

        // Video Output
        .output(this.videoOutputURL)
        .videoCodec("libx264")
        .outputFormat("mpegts")
        .outputOptions([
          "-map 0:v",
          "-preset ultrafast"
        ])

        // Audio Output
        .output(this.audioOutputURL)
        .audioCodec("aac")
        .outputFormat("mpegts")
        .outputOptions([
          "-map 1:a",
        ]);

      this.ffmpeg.run();
    }
    catch (err) {
      this.error(err);
      this.retry();
    }
  }
}
