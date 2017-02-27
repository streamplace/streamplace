
import winston from "winston";
import _ from "underscore";

export default function(ctx = {}, details = "") {
  let logBits = [];
  if (ctx.remoteAddress) {
    logBits.push(`${ctx.remoteAddress}`);
  }
  if (ctx.user) {
    logBits.push(`[${ctx.user.handle}]`);
  }
  // Lots of harmless polling... this will be a verbosity-level thing in the future but for now...
  if (ctx.user && _(ctx.user.roles).contains("SERVICE") && details.indexOf("GET") !== -1) {
    return;
  }
  /*eslint-disable no-console */
  console.log(logBits.join(" "), details);
}
