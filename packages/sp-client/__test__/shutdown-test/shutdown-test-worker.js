/* eslint-disable no-console */
const SPClient = require("../../dist/sp-client").SPClient;

Array(10)
  .fill(true)
  .forEach(() => {
    const SP = new SPClient();

    SP.connect().then(() => {
      console.log("connected, disconnecting...");

      SP.inputs.watch({}).then(() => {
        console.log("data event");
        SP.disconnect();
      });
    });
  });
