
import chokidar from "chokidar";
import fs from "fs";
import {warn} from "./log";

let ignored = [];
try {
  const gitignore = fs.readFileSync(".gitignore", "utf8");
  ignored = ignored.concat(gitignore.split("\n"));
}
catch (e) {
  if (e.code !== "ENOENT") {
    throw(e);
  }
  warn("You don't have a .gitignore file, that's weird.");
}

export default function(config) {
  chokidar.watch('.', {ignored}).on('all', (event, path) => {
    console.log(event, path);
  });
}
