
import net from "net";
import express from "express";
import morgan from "morgan";
import bodyParser from "body-parser";
import ffmpeg from "fluent-ffmpeg";
import bunyan from "bunyan";
import stream from "stream";

const log = bunyan.createLogger({name: 'pipeland'});
const app = express();
const LOG_LEVEL_NONE = 0;
const LOG_LEVEL_ERROR = 1;
const LOG_LEVEL_WARNING = 2;
const LOG_LEVEL_INFO = 3;
const LOG_LEVEL_DEBUG = 4;
const LOG_LEVEL = LOG_LEVEL_DEBUG;

app.use(morgan('combined'));
app.use(bodyParser.urlencoded({
  extended: false
}));

app.post('*', function(req, res) {
  if (req.body) {
    const data = req.body;
    log.info(`nginx says: ${data.addr} is doing ${data.call} at ${data.tcurl}/${data.name} (${data.flashver})`);
    res.status(200).end();
    if (req.body.call === 'publish') {
      addSource(req.body);
    }
  }
});

app.listen(3000);

let streamCount = 0;

const wrappedffmpeg = function(name) {

  const streamLog = bunyan.createLogger({name: `${name} (${streamCount})`});
  streamLog.level("trace");
  streamCount += 1;

  return ffmpeg({
    logger: {
      error: streamLog.error,
      warning: streamLog.warn,
      info: streamLog.info,
      debug: streamLog.debug,
    }
  })
    .on('error', function(err, stdout, stderr) {
      streamLog.error('fired event: error');
      streamLog.error("err", err);
      streamLog.error("stdout", stdout);
      streamLog.error("stderr", stderr);
    })
    .on('codecData', function(data) {
      streamLog.info('fired event: codecData', data);
    })
    .on('end', function() {
      streamLog.info('fired event: end');
    })
    .on('progress', function(data) {
      streamLog.trace('fired event: progress');
      streamLog.trace(data);
    })
    .on('start', function(command) {
      streamLog.info('fired event: start');
      streamLog.info('command: ' + command);
    });
};

let outputStream;
let oldInputProcess;
let oldInputStream;

const addSource = function(data) {
  const appPath = data.tcurl.split('/').pop();
  const inputUrl = `rtmp://localhost:1955/${appPath}/${data.name}`;
  const outputUrl = `rtmp://localhost:1934/${appPath}/test`;
  log.info(`Streaming from ${inputUrl} to ${outputUrl}`);

  if (!outputStream) {
    log.info("Setting up output stream");
    outputStream = new stream.PassThrough();

    wrappedffmpeg('output')
      .input(outputStream)
      .inputFormat('mpegts')
      // .inputFormat('ismv')
      .videoCodec('copy')
      .outputFormat('flv')
      .noAudio()
      .save(outputUrl);
  }

  const inputProcess = wrappedffmpeg('input')
    .input(inputUrl)
    .noAudio()
    .inputFormat('flv')
    .outputOptions(['-bsf:v h264_mp4toannexb'])
    .videoCodec('copy')
    .outputFormat('mpegts')

  const inputStream = inputProcess.stream();


  log.info("Got new stream, will replace when we get data.");
  let replaced = false;

  inputStream.on('data', function(chunk) {
    if (!replaced) {
      replaced = true;
      if (oldInputProcess) {
        oldInputStream.pause();
        oldInputProcess.kill();
      }
      oldInputStream = inputStream;
      oldInputProcess = inputProcess;
    }
    outputStream.push(chunk);
  });

};

log.info("Pipeland booting up.");

// const PORT = 1730;
// const HOST = "drumstick.iame.li";
// const REMOTE_PORT = 1934;

// const output = new net.Socket({
//   readable: true,
//   writable: true
// });

// const server = net.createServer(function(incoming) {
//   console.log(`Opening connection to ${HOST}:${REMOTE_PORT}`);
//   const outgoing = net.createConnection({
//     readable: true,
//     writable: true,
//     host: HOST,
//     port: REMOTE_PORT
//   });
//   incoming.pipe(outgoing);
//   outgoing.pipe(incoming);
// });

// console.log(`Listening on ${PORT}`);
// server.listen(PORT, "127.0.0.1");

