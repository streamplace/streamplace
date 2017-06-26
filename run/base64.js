// base64 is inconsistent between linux and mac so let's just do it here
const getStdin = require("get-stdin");

/* eslint-disable no-console */
getStdin().then(str => {
  if (process.argv[2] === "--decode") {
    process.stdout.write(Buffer.from(str, "base64").toString());
  } else {
    process.stdout.write(Buffer.from(str).toString("base64"));
  }
});
