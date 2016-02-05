
import SKClient from "sk-client";

import ENV from "./env";

SKClient.log(true);

export default new SKClient({server: "http://localhost:80"});
