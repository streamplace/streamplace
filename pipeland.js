
import net from "net";
import express from "express";
import morgan from "morgan";
import bodyParser from "body-parser";
import ffmpeg from "fluent-ffmpeg";

const app = express();

app.use(morgan('combined'));
app.use(bodyParser.urlencoded({
  extended: false
}));

app.post('*', function(req, res) {
  if (req.body) {
    // console.log(req.body);
    res.status(200).end();
    if (req.body.call === 'publish') {
      addSource(req.body);
    }
  }
});

app.listen(3000);

const doErr = function(name) {
  return function(err, stdout, stderr) {
    console.log('ffmpeg error in ' + name);
    console.log(err);
    console.log(stdout);
    console.log(stderr);
  }
};

const addSource = function(data) {
  const appPath = data.swfurl.split('/').pop();
  const inputUrl = `rtmp://localhost:1955/${appPath}/${data.name}`;
  const outputUrl = `rtmp://localhost:1934/${appPath}/${data.name}`;
  console.log(`Streaming from ${inputUrl} to ${outputUrl}`);

  const inputStream = ffmpeg(inputUrl)
    .videoCodec('copy')
    .audioCodec('copy')
    .on('error', doErr('input stream'))
    .outputFormat('flv')
    .stream();

  ffmpeg(inputStream)
    .videoCodec('copy')
    .audioCodec('copy')
    .on('error', doErr('output stream'))
    .outputFormat('flv')
    .save(outputUrl);
};

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

