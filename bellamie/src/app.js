
import express from "express";
import winston from "winston";

import ENV from "./env";

const app = express();

if (!module.parent) {
  app.listen(ENV.PORT);
  winston.info("Bellamie starting up on port " + ENV.PORT);
}
