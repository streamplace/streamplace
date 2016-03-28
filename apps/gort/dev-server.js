
/*eslint-disable no-var */
var path = require("path");
var express = require("express");
var morgan = require("morgan");
var app = express();

// Log stuff
app.use(morgan("combined"));

// Serve files if they exist
app.use(express.static(__dirname));

// Failing that, give 'em index.html
app.use(function(req, res) {
  res.sendFile(path.resolve(__dirname, "index.html"));
});

app.listen(process.env.PORT);
