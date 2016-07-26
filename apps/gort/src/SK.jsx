
import SKClient from "sk-client";
import config from "sk-config";

const server = config.require("PUBLIC_API_SERVER_URL");

const SK = new SKClient({
  server: server,
  log: true,
  start: false
});

export default SK;

// Ehhhhh. Makes debugging easier. Who cares.
window.SK = SK;
