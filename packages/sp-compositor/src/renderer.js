const { desktopCapturer } = require("electron");
const url = require("url");
const querystring = require("querystring");
const fs = require("fs");
const path = require("path");

const options = querystring.parse(url.parse(document.location.href).query);
const video = document.querySelector("video");
video.style.width = `${options.width}px`;
video.style.height = `${options.height}px`;

const writeStream = fs.createWriteStream(
  path.resolve(__dirname, "..", `${options.windowId}.webm`)
);

desktopCapturer.getSources(
  { types: ["window", "screen"] },
  (error, sources) => {
    const source = sources.find(source => source.name === options.windowId);

    navigator.mediaDevices
      .getUserMedia({
        audio: false,
        video: {
          mandatory: {
            chromeMediaSource: "desktop",
            chromeMediaSourceId: source.id,
            minWidth: options.width,
            maxWidth: options.width,
            minHeight: options.height,
            maxHeight: options.height
          }
        }
      })
      .then(stream => {
        // video.srcObject = stream;
        const recorder = new MediaRecorder(stream);
        window.recorder = recorder;
        recorder.ondataavailable = event => {
          var fileReader = new FileReader();
          fileReader.onload = function() {
            writeStream.write(Buffer.from(this.result));
          };
          fileReader.readAsArrayBuffer(event.data);
        };
        recorder.onerror = (...args) => console.log("onerror", args);
        recorder.onpause = (...args) => console.log("onpause", args);
        recorder.onresume = (...args) => console.log("onresume", args);
        recorder.onstart = (...args) => console.log("onstart", args);
        recorder.onstop = (...args) => console.log("onstop", args);
        recorder.start(100);
      })
      .catch(err => {
        console.log("getUserMedia error", err);
      });

    // if (error) throw error;
    // for (let i = 0; i < sources.length; ++i) {
    //   if (sources[i].name === "Electron") {
    //
    //     return;
    //   }
    // }
  }
);
