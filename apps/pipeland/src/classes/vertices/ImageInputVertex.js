
import temp from "temp";
import request from "request";
import url from "url";
import path from "path";

import InputVertex from "./InputVertex";
import SK from "../../sk";

// We want to download the file locally first. FFmpeg will download it over and over when looping
// otherwise.
temp.dir = "/tmp"; // os.tmpDir is set weird in Docker. Hack hack hack.
temp.track();

const fetchFile = function(fileURL) {
  return new Promise((resolve, reject) => {
    const {pathname} = url.parse(fileURL); // gives /pub/whatever/example.png
    const basename = path.basename(pathname); // gives example.png
    const writeStream = temp.createWriteStream({suffix: basename});
    const outputPath = writeStream.path;
    writeStream
      .on("error", reject)
      .on("finish", () => {
        resolve(outputPath);
      });

    request.get(fileURL)
      .on("error", reject)
      .pipe(writeStream);
  });
};

export default class ImageInputVertex extends InputVertex {
  constructor({id}) {
    super({id});
    this.streamFilters = ["sync"];
    // this.debug = true;
    this.videoOutputURL = this.transport.getOutputURL();
    SK.vertices.update(id, {
      outputs: [{
        name: "default",
        sockets: [{
          url: this.videoOutputURL,
          type: "video"
        }]
      }]
    })
    .then((doc) => {
      this.doc = doc;
      return fetchFile(this.doc.params.url);
    })
    .then((filePath) => {
      this.filePath = filePath;
      this.info(`Downloaded ${filePath}`);
      this.init();
    })
    .catch((err) => {
      this.error(err);
    });
  }

  init() {
    super.init();
    try {
      this.ffmpeg = this.createffmpeg()
        .input(this.filePath)
        .inputFormat("image2")
        .inputOptions([
          "-loop 1",
          "-re"
        ])

        // Video out
        .output(this.videoOutputURL)
        .videoCodec("libopenh264")
        .outputOptions([
          "-map 0:v"
        ])
        .outputFormat("mpegts");

      this.ffmpeg.run();
    }
    catch (err) {
      this.error(err);
      this.retry();
    }
  }
}
