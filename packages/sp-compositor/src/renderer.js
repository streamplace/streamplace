const { desktopCapturer } = require("electron");
const url = require("url");
const querystring = require("querystring");
const fs = require("fs");
const path = require("path");
const { tcpEgressStream } = require("sp-streams");

/* eslint-disable no-console */

const options = querystring.parse(url.parse(document.location.href).query);
const video = document.querySelector("video");
video.style.width = `${options.width}px`;
video.style.height = `${options.height}px`;

desktopCapturer.getSources(
  { types: ["window", "screen"] },
  (error, sources) => {
    if (error) {
      throw error;
    }
    let source = sources.find(source => source.name === options.windowId);
    if (!source) {
      if (sources.length > 0) {
        console.log(
          `Couldn't find ${options.windowId}, falling back to ${JSON.stringify(
            sources[0]
          )}`
        );
        source = sources[0];
      } else {
        throw new Error("No sources received from desktopCapturer");
      }
    }

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
        const tcpEgress = tcpEgressStream({ port: options.port });
        let counter = 0;
        recorder.ondataavailable = event => {
          let fileReader = new FileReader();
          let me = counter;
          counter += 1;
          fileReader.onload = function() {
            try {
              tcpEgress.write(Buffer.from(this.result));
            } catch (e) {
              console.error(e);
            }
          };
          fileReader.readAsArrayBuffer(event.data);
        };
        // recorder.onerror = (...args) => console.log("onerror", args);
        // recorder.onpause = (...args) => console.log("onpause", args);
        // recorder.onresume = (...args) => console.log("onresume", args);
        // recorder.onstart = (...args) => console.log("onstart", args);
        // recorder.onstop = (...args) => console.log("onstop", args);
        setTimeout(() => {
          console.log("starting");
          recorder.start(100);
        }, 5000);
      })
      .catch(err => {
        console.log("getUserMedia error");
        console.log(err);
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
