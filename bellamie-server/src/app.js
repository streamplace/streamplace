
import logger from "koa-logger";
import router from "koa-router";
import koa from "koa";
import winston from "winston";

import ENV from "./env";
import broadcasts from "./resources/broadcasts";

const app = koa();

export default app;

const routes = router();
routes.use("/broadcasts", broadcasts.routes());

app.use(logger());
app.use(routes.routes());

if (!module.parent) {
  app.listen(ENV.PORT);
  winston.info("Bellamie starting up on port " + ENV.PORT);
}
