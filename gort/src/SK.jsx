import SKClient from "sk-client";

const SK = new SKClient({
  server: "http://drumstick.iame.li:8100",
  log: true
});

export default SK;

// Ehhhhh. Makes debugging easier. Who cares.
window.SK = SK;
