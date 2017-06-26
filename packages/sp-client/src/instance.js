import SKClient from "./SKClient";
import config from "sk-config";

const SK = new SKClient({
  log: true,
  start: false
});

export default SK;

// Ehhhhh. Makes debugging easier. Who cares.
if (typeof window === "object") {
  window.SK = SK;
}
