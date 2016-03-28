require("babel-register")({
  only: /src/,
});
module.exports = require("./src/sk-client");
