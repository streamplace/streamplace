
const JSONStream = require("JSONStream");
const request = require("request");
const getColor = require("../../run/get-color");
const term = require("terminal-kit").terminal;

request({
  url: process.argv[2],
  headers: {
    "Accept": "application/json",
  },
})
.pipe(JSONStream.parse())
.on("data", function(obj) {
  let hostname = obj.Container.Config.Hostname;
  // Parse out the correct color from the pod name if we can
  let color;
  if (hostname.indexOf("dev-") === 0) {
    const [_, podName, suffix] = hostname.match(/^dev-([a-z-]+)(-.+$)/);
    color = getColor(podName);
    hostname = podName;
  }
  else {
    color = getColor(hostname);
  }
  term.color256(color)(hostname);
  term.styleReset(` ${obj.Data}\n`);
});
