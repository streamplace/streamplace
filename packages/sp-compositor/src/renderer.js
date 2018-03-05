const debug = require("debug");

const log = debug("sp:sp-compositor-renderer");

const { desktopCapturer } = require("electron");
const url = require("url");
const querystring = require("querystring");
const fs = require("fs");
const path = require("path");
const { tcpEgressStream, rtmpOutputStream } = require("sp-streams");
import convertVideoStream from "./convert-video-stream";

/* eslint-disable no-console */

const options = querystring.parse(url.parse(document.location.href).query);
log(options);
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
        let output;
        if (options.rtmp) {
          output = new convertVideoStream();
          output.pipe(rtmpOutputStream({ rtmpUrl: options.rtmp }));
        } else {
          output = tcpEgressStream({ port: options.port });
        }
        recorder.ondataavailable = event => {
          let fileReader = new FileReader();
          fileReader.onload = function() {
            try {
              output.write(Buffer.from(this.result));
            } catch (e) {
              console.error(e);
            }
          };
          fileReader.readAsArrayBuffer(event.data);
        };
        recorder.onerror = (...args) => log("onerror", args);
        recorder.onpause = (...args) => log("onpause", args);
        recorder.onresume = (...args) => log("onresume", args);
        recorder.onstart = (...args) => log("onstart", args);
        recorder.onstop = (...args) => log("onstop", args);
        setTimeout(() => {
          log("starting");
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
