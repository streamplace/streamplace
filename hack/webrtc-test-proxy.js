
import http from "http";
import net from "net";

const conn = net.createConnection({port: 9998, host: "localhost"});

const server = http.createServer((req, res) => {
  console.log("hit")
  req.on("end", function() {
    console.log("end");
    res.end();
  })
  req.on("data", function(chunk) {
    conn.write(chunk);
  });
  res.end();
});

server.on('clientError', (err, socket) => {
  console.log(err);
  socket.end('HTTP/1.1 400 Bad Request\r\n\r\n');
});

server.listen(9999);
