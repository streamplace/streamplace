
import "babel-polyfill";
import logger from "koa-logger";
import route from "koa-route";
import koa from "koa";

const app = koa();

export default app;

app.use(logger());
app.use(function * welcome(next) {
  this.body = "Welcome to Bellamie!";
});

if (!module.parent) {
  app.listen(80);
}
