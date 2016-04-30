
import SKClient from "sk-client";

import ENV from "./env";

export default new SKClient({
  server: ENV.API_SERVER_URL,
  log: true
});
