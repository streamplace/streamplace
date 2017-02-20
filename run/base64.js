
// base64 is inconsistent between linux and mac so let's just do it here
const getStdin = require("get-stdin");

/* eslint-disable no-console */
getStdin().then(str => {
  console.log(Buffer.from(str).toString("base64"));
});
