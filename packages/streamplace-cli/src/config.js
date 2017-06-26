import { safeLoad as readYaml, safeDump as writeYaml } from "js-yaml";
import { sync as mkdirp } from "mkdirp-promise";
import { dirname } from "path";
import fs from "mz/fs";

const CONFIG_DEFAULT = {
  authServer: "https://stream.place"
};

export default function getConfig(program) {
  const { spConfig } = program;
  let stat;
  try {
    stat = fs.statSync(spConfig);
  } catch (e) {
    // Create if it doesn't exist.
    if (e.code !== "ENOENT") {
      throw e;
    }
    mkdirp(dirname(spConfig));
    fs.writeFileSync(spConfig, writeYaml(CONFIG_DEFAULT));
  }
  const data = fs.readFileSync(spConfig, "utf8");
  return readYaml(data);
}
