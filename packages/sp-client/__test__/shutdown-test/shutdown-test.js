// Currently fighting a bug where SPClient keeps node running even after we do
// SPClient.disconnect. No good! This makes sure that's not happening.
/* eslint-disable no-console */
const resolve = require("path").resolve;
const spawn = require("child_process").spawn;

const proc = spawn("node", [resolve(__dirname, "shutdown-test-worker.js")], {
  stdio: "inherit"
});

const TIMEOUT = 5000;

proc.on("exit", code => {
  if (code === 0) {
    console.info("subprocess exited in a timely fashion, hooray!");
    process.exit(0);
  }
  console.error("shutdown-test-worker exited with code " + code);
  process.exit(code);
});

setTimeout(function() {
  console.error(`shutdown-test-worker hasn't exited after ${TIMEOUT}ms`);
  proc.kill("SIGKILL");
  process.exit(1);
}, TIMEOUT);
