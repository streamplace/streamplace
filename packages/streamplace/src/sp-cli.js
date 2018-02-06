import { version } from "../package.json";
import yargs from "yargs";
import debug from "debug";
import { spStream } from "sp-streams";

const log = debug("sp:cli");

export default function cli() {
  const argv = yargs.demandCommand(1).argv;
  return spStream({ filePath: argv._[0] });
}
