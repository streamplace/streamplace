const { desktopCapturer } = require("electron");
const url = require("url");
const querystring = require("querystring");

const options = querystring.parse(url.parse(document.location.href).query);
const video = document.querySelector("video");
video.style.width = `${options.width}px`;
video.style.height = `${options.height}px`;

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
        video.src = URL.createObjectURL(stream);
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
