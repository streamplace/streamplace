/* eslint-disable no-console */

const NodeConsole = require("console").Console;
const nodeConsole = new NodeConsole(process.stdout, process.stderr);
import { parseToRgb } from "polished";
import style from "ansi-styles";

const COLOR_PREFIX = "color :";

const makeLogger = function(func) {
  return function(first, ...rest) {
    if (typeof first !== "string") {
      return func(first, ...rest);
    }
    rest = rest.filter(arg => {
      if (typeof arg !== "string") {
        return true;
      }
      if (!arg.startsWith("color: ")) {
        return true;
      }
      let code;
      let color = arg.substr(COLOR_PREFIX.length);
      if (color === "inherit") {
        code = style.color.close;
      } else {
        try {
          const { red, green, blue } = parseToRgb(color);
          code = style.color.ansi256.rgb(red, green, blue);
        } catch (e) {
          code = style.color.close;
        }
      }
      first = first.replace("%c", code);
      return false;
    });
    first += style.color.close;
    return func(first, ...rest);
  };
};

console.log = makeLogger(nodeConsole.log.bind(nodeConsole));
console.error = makeLogger(nodeConsole.error.bind(nodeConsole));

window.addEventListener("unhandledrejection", function(event) {
  console.error(
    "Unhandled rejection (promise: ",
    event.promise,
    ", reason: ",
    event.reason,
    ")."
  );
});
// console.error = (...args) => {
//   nodeConsole.error(args);
// };
// console.log = nodeConsole.log.bind(nodeConsole);
// console.error = nodeConsole.error.bind(nodeConsole);
