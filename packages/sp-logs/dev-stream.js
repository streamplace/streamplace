const JSONStream = require("JSONStream");
const request = require("request");
const getColor = require("../../run/get-color");
const term = require("terminal-kit").terminal;
const yaml = require("js-yaml");
const fs = require("fs");
const path = require("path");

const data = yaml.safeLoad(
  fs.readFileSync(path.resolve(__dirname, "..", "..", "values-dev.yaml"))
);

const logUrl = `http://${data.global.domain}/logs/logs`;

const BLACKLIST = ["moby", "linuxkit"];

const retry = function() {
  const TIMEOUT = 5000;
  term(`Trying again in ${TIMEOUT}ms...\n`);
  setTimeout(makeAttempt, TIMEOUT);
};

process.on("uncaughtException", err => {
  term(`${err}\n`);
  myRetry();
});

let didRetry;
let myRetry;

const makeAttempt = function() {
  didRetry = false;
  myRetry = () => {
    if (didRetry) {
      // console.log("canceled dupe retry");
      return;
    }
    didRetry = true;
    retry();
  };
  try {
    request({
      url: logUrl,
      headers: {
        Accept: "application/json"
      }
    })
      .on("error", function(err) {
        term(`${err}\n`);
        myRetry();
      })
      .on("end", function(err) {
        term("log stream ended\n");
        myRetry();
      })
      .on("response", function() {
        term("log stream opened\n");
      })
      .pipe(JSONStream.parse())
      .on("data", function(obj) {
        let hostname = obj.Container.Config.Hostname;
        if (BLACKLIST.some(bad => hostname.includes(bad))) {
          return;
        }
        if (hostname.includes("kube-dns")) {
          return;
        }
        // Parse out the correct color from the pod name if we can
        let color;
        if (hostname.indexOf("dev-") === 0) {
          const [_, podName, suffix] = hostname.match(/^dev-([a-z-]+)(-.+$)/);
          color = getColor(podName);
          hostname = podName;
        } else {
          color = getColor(hostname);
        }
        term.color256(color)(hostname);
        term.styleReset(` ${obj.Data}\n`);
      });
  } catch (e) {
    term(`${e}\n`);
    myRetry();
  }
};

makeAttempt();
