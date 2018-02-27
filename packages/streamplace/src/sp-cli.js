import { version } from "../package.json";
import Yargs from "yargs";
import debug from "debug";
import { spStream } from "sp-streams";

const log = debug("sp:cli");

/* eslint-disable no-console */

export default async function cli() {
  const yargs = Yargs.options({
    verbose: {
      describe: "enable lots of additional logging",
      default: false,
      type: "boolean"
    },
    file: {
      alias: "f",
      describe: "path to a file you'd like to stream",
      type: "string"
    },
    loop: {
      describe: "loop the input file forever",
      type: "boolean"
    }
  });
  const argv = yargs.argv;
  process.on("unhandledRejection", err => {
    if (
      typeof err.status === "number" &&
      err.status > 399 &&
      err.status < 500
    ) {
      console.error(err.message);
      yargs.showHelp();
      process.exit(1);
    }
    console.log("fatal error - unhandled rejection");
    err && console.log(err);
    process.exit(1);
  });
  if (!argv.file) {
    const err = new Error(`Usage: ${process.argv[1]} -f [file]`);
    err.status = 400;
    throw err;
  }
  const stream = await spStream({ filePath: argv.file, loop: argv.loop });
  console.log(`streaming to ${stream.url}`);
  stream.on("end", () => {
    process.exit(0);
  });
}
