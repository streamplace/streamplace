/* eslint-disable no-console */
/* global nw */

const env = {
  URL: null,
  SELECTOR: null,
  POLL_TIMEOUT: "60000"
};

const windowOptions = {
  title: "Streamplace Compositor",
  width: 1920,
  height: 1080,
  min_width: 1920,
  min_height: 1080,
  resizable: true,
  frame: true
};

let quit = false;
Object.keys(env).forEach(key => {
  if (process.env[key]) {
    env[key] = process.env[key];
  }
  if (!env[key]) {
    quit = true;
    console.error(`Missing required environment variable: ${key}`);
  }
});
if (quit) {
  process.exit(1);
}

const pollForElement = function(document, selector) {
  return new Promise((resolve, reject) => {
    const timeout = setTimeout(() => {
      clearInterval(interval);
      reject(
        `Couldn't find document.querySelector("${selector}") in ${env.POLL_TIMEOUT}ms!`
      );
    }, parseInt(env.POLL_TIMEOUT));
    const poll = () => {
      const elem = document.querySelector(selector);
      if (elem) {
        clearTimeout(timeout);
        clearInterval(interval);
        resolve(elem);
      }
    };
    const interval = setInterval(poll, 10);
    poll();
  });
};

nw.Window.open(process.env.URL, windowOptions, function(new_win) {
  // And listen to new window's focus event
  const worker = new Worker("dist/worker.js");
  new_win.on("close", function() {
    process.exit(0);
  });
  new_win.on("loaded", function() {
    const document = new_win.window.document;
    console.log(
      `Window loaded, polling for document.querySelector("${env.SELECTOR}")`
    );
    const start = Date.now();
    pollForElement(document, env.SELECTOR)
      .then(elem => {
        const gl = elem.getContext("experimental-webgl", {
          preserveDrawingBuffer: true
        });
        const pixels = new Uint8Array(
          gl.drawingBufferWidth * gl.drawingBufferHeight * 4
        );
        const run = function() {
          setTimeout(run, 5);
          if (new_win.window.newFrame === true) {
            gl.readPixels(
              0,
              0,
              gl.drawingBufferWidth,
              gl.drawingBufferHeight,
              gl.RGBA,
              gl.UNSIGNED_BYTE,
              pixels
            );
            worker.postMessage(pixels);
            new_win.window.newFrame = false;
          }
        };
        run();
      })
      .catch(err => {
        console.error(err);
        process.exit(1);
      });
  });
});
