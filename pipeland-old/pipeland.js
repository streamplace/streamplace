
import net from "net";
import express from "express";
import morgan from "morgan";
import bodyParser from "body-parser";
import ffmpeg from "fluent-ffmpeg";
import bunyan from "bunyan";
import stream from "stream";
import http from "http";
import path from "path";
import ioPackage from "socket.io";

const log = bunyan.createLogger({name: "pipeland"});
const app = express();

app.use(morgan("combined"));
app.use(bodyParser.urlencoded({
  extended: false
}));

app.post("*", function(req, res) {
  if (req.body) {
    const data = req.body;
    log.info(`nginx says: ${data.addr} is doing ${data.call} at ${data.tcurl}/${data.name} (${data.flashver})`);
    res.status(200).end();
    if (req.body.call === "publish") {
      addSource(req.body);
    }
  }
});

app.get("*", function(req, res) {
  res.sendFile(path.resolve(__dirname, "switcher.html"));
});

const server = require("http").Server(app);
server.listen(3000);

let streamCount = 0;

const wrappedffmpeg = function(name) {

  const streamLog = bunyan.createLogger({name: `${name} (${streamCount})`});
  streamCount += 1;

  return ffmpeg({
    logger: {
      error: streamLog.error,
      warning: streamLog.warn,
      info: streamLog.info,
      debug: streamLog.debug,
    }
  })
    .on("error", function(err, stdout, stderr) {
      streamLog.error("fired event: error");
      streamLog.error("err", err);
      streamLog.error("stdout", stdout);
      streamLog.error("stderr", stderr);
    })
    .on("codecData", function(data) {
      streamLog.info("fired event: codecData", data);
    })
    .on("end", function() {
      streamLog.info("fired event: end");
    })
    .on("progress", function(data) {
      streamLog.trace("fired event: progress");
      streamLog.trace(data);
    })
    .on("start", function(command) {
      streamLog.info("fired event: start");
      streamLog.info("command: " + command);
    });
};

let currentStream = 0;
let inputStreams = [];
const renderStreams = function() {
  return inputStreams.map((obj) => {
    return {
      name: obj.name,
    };
  });
};

const sockets = [];
const io = ioPackage(server);
io.on("connection", function(socket) {

  socket.emit("streams", renderStreams());
  socket.emit("activeStream", currentStream);

  socket.on("switch", function(idx) {
    idx = parseInt(idx);
    // console.log("Setting stream to " + idx);
    currentStream = idx;
    notifyAll("activeStream", currentStream);
  });

  socket.on("disconnect", function() {
    sockets.splice(sockets.indexOf(socket), 1);
  });

  sockets.push(socket);
});

const notifyAll = function(...args) {
  sockets.forEach(function (socket) {
    socket.emit(...args);
  });
};

let outputStream;


const addSource = function(data) {
  const appPath = data.tcurl.split("/").pop();
  const inputUrl = `rtmp://localhost:1955/${appPath}/${data.name}`;
  const outputUrl = `rtmp://localhost:1934/${appPath}/test`;
  log.info(`Streaming from ${inputUrl} to ${outputUrl}`);

  if (!outputStream) {
    log.info("Setting up output stream");
    outputStream = new stream.PassThrough();

    wrappedffmpeg("output")
      .input(outputStream)
      .inputFormat("mpegts")
      // .inputFormat("ismv")
      .audioCodec("aac")
      .videoCodec("copy")
      // .outputOptions([
      //   "-bsf:a aac_adtstoasc"
      // ])
      .outputFormat("flv")
      .save(outputUrl);
  }

  const inputProcess = wrappedffmpeg("input")
    .input(inputUrl)
    .inputFormat("flv")
    .outputOptions(["-bsf:v h264_mp4toannexb"])
    .videoCodec("libx264")
    .audioCodec("libmp3lame")
    .outputOptions([
      "-preset ultrafast",
      "-tune zerolatency",
      "-x264opts keyint=5:min-keyint="
    ])
    .outputFormat("mpegts");

  const inputStream = inputProcess.stream();
  inputStreams.push({
    stream: inputStream,
    name: data.name,
  });

  notifyAll("streams", renderStreams());

  const myNumber = inputStreams.length - 1;

  log.info("Got new stream, number is " + myNumber);
  // let replaced = false;

  inputStream.on("data", function(chunk) {
    // if (!replaced) {
    //   replaced = true;
    //   if (oldInputProcess) {
    //     oldInputStream.pause();
    //     oldInputProcess.kill();
    //   }
    //   oldInputStream = inputStream;
    //   oldInputProcess = inputProcess;
    // }
    if (currentStream === myNumber) {
      outputStream.push(chunk);
    }
  });

};

log.info("Pipeland booting up.");
