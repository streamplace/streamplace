
import chokidar from "chokidar";
import fs from "mz/fs";
import {info, warn, error} from "./log";
import WebSocket from "ws";
import debugPkg from "debug";

const debug = debugPkg("sp:cli-sync");

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

// TODO on this: Handle add/change/unlink events in quick succession
export class FileSyncClient {
  constructor({directory, packageManagerUrl}) {
    this.ws = new WebSocket("ws://localhost:8999");

    this.ws.on("open", () => {
      info("Connection established.");
      this.jsonMessage({event: "hello"});
      this.ready = true;
      this._processNext();
    });

    this.messageQueue = [];
    this.ready = false;
  }

  /**
   * Add a monitored file event to the queue.
   */
  queue(event, path) {
    if (event === "add" || event === "change") {
      fs.readFile(path).then((buf) => {
        debug(`Got ${buf.length} bytes for ${path}`);
        this.messageQueue.push({event, path, buf});
        this._processNext();
      })
      .catch((err) => {
        error(err);
      });
    }

    this._processNext();
  }

  /**
   * Do the next thing if there's a next thing worth doing
   */
  _processNext() {
    if (this.messageQueue.length === 0) {
      return;
    }
    if (!this.ready) {
      return;
    }
    const {event, path, buf} = this.messageQueue.shift();
    this.ws.send(JSON.stringify({event, path}));
    this.ws.send(buf);
  }

  /**
   * Sends a multi-part ws message containing a metadata message and some optional buffers
   * @param  {Object}    message     [description]
   * @param  {...[Buffer]} attachments [description]
   */
  jsonMessage(message, ...attachments) {
    message.attachments = attachments.length;
    this.ws.send(JSON.stringify(message));
    attachments.forEach((attachment) => {
      this.ws.send(attachment);
    });
  }
}




export default function(config) {
  const fileSync = new FileSyncClient({
    directory: ".",
    packageManagerUrl: "ws://localhost:8999",
  });
  chokidar.watch(".", {ignored}).on("all", (event, path) => {
    debug(`Got ${event} for ${path}`);
    fileSync.queue(event, path);
  });
}
