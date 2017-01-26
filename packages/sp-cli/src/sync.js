
// import chokidar from "chokidar";
// import fs from "mz/fs";
// import {info, warn, error} from "./log";
// import WebSocket from "ws";
// import debugPkg from "debug";
// import vorpalPkg from "vorpal";
// import inquirer from "inquirer";
// import {terminal as term} from "terminal-kit";

import {createStore, combineReducers, applyMiddleware} from "redux";
import thunk from "redux-thunk";

import {STARTUP} from "./constants/actionNames";
import terminalComponent from "./terminal/terminalComponent";
import watcherComponent from "./watcher/watcherComponent";
import rootReducer from "./reducer";

export default function() {
  const store = createStore(rootReducer, applyMiddleware(thunk));
  terminalComponent(store);
  watcherComponent(store);
}


// term.fullscreen();

// let ignored = [];
// try {
//   const gitignore = fs.readFileSync(".gitignore", "utf8");
//   ignored = ignored.concat(gitignore.split("\n"));
// }
// catch (e) {
//   if (e.code !== "ENOENT") {
//     throw(e);
//   }
//   warn("You don't have a .gitignore file, that's weird.");
// }

// term.moveTo(1, term.height);

// let underline = "";
// while (underline.length < term.width) {
//   underline += " ";
// }

// const debug = function(str) {
//   term.cyan("[debug] ");
//   term(str);
//   term("\n");
//   frame();
// };

// const frame = function() {
//   term.saveCursor();
//   term.moveTo(1, 1);
//   term(underline);
//   term.moveTo(1, 2);
//   term(underline);
//   term.moveTo(1, 1);
//   term.bgColorRgb(0, 0, 0).yellow(" Streamplace");
//   let statusString = conn ? "Connected" : "Disconnected";
//   const position = term.width - statusString.length ;
//   term.moveTo(position, 1);
//   if (conn) {
//     term.green("Connected\n");
//   }
//   else {
//     term.red("Disconnected\n");
//   }
//   term.underline(underline);
//   term.restoreCursor();
// };

// let conn = false;

// const connected = function(status) {
//   conn = status;
//   frame();
// };

// // TODO on this: Handle add/change/unlink events in quick succession
// export class FileSyncClient {
//   constructor({directory, packageManagerUrl}) {
//     this.ready = false;
//     this.messageQueue = [];
//     this.initWs();
//   }

//   initWs() {
//     this.ws = new WebSocket("ws://localhost:8999");

//     this.ws.on("open", () => {
//       debug("Connection established.");
//       this.jsonMessage({event: "hello"});
//       this.ready = true;
//       this.active = false;
//       this._processNext();
//       connected(true);
//     });

//     this.ws.on("close", (code, reason) => {
//       debug("close", code, reason);
//       connected(false);
//     });

//     this.ws.on("error", (err) => {
//       debug("error", err);
//     });

//     this.ws.on("message", (err) => {
//       debug("message", err);
//     });

//     this.ws.on("ping", (data, flags) => {
//       debug("ping", data, flags);
//     });

//     this.ws.on("pong", (data, flags) => {
//       debug("pong", data, flags);
//     });

//     this.ws.on("unexpected-response", (request, response) => {
//       debug("unexpected-response", request, response);
//     });
//   }

//   /**
//    * Add a monitored file event to the queue.
//    */
//   queue(event, path) {
//     this.messageQueue.push({event, path});
//     this._processNext();
//   }

//   /**
//    * Do the next thing if there's a next thing worth doing
//    */
//   _processNext() {
//     if (this.messageQueue.length === 0) {
//       return;
//     }
//     if (!this.ready || this.active) {
//       return;
//     }
//     const {event, path, buf} = this.messageQueue.shift();
//     if (event === "add" || event === "change") {
//       this.active = true;
//       fs.readFile(path).then((buf) => {
//         debug(`Got ${buf.length} bytes for ${path}`);
//         this.jsonMessage({event, path}, buf);
//         this.active = false;
//         this._processNext();
//       })
//       .catch((err) => {
//         error(err);
//         process.exit(1);
//       });
//     }
//   }

//   /**
//    * Sends a multi-part ws message containing a metadata message and some optional buffers
//    * @param  {Object}    message     [description]
//    * @param  {...[Buffer]} attachments [description]
//    */
//   jsonMessage(message, ...attachments) {
//     message.attachments = attachments.length;
//     this.ws.send(JSON.stringify(message));
//     attachments.forEach((attachment) => {
//       this.ws.send(attachment);
//     });
//   }
// }

// export default function(config) {
//   const fileSync = new FileSyncClient({
//     directory: ".",
//     packageManagerUrl: "ws://localhost:8999",
//   });
//   chokidar.watch(".", {ignored}).on("all", (event, path) => {
//     debug(`Got ${event} for ${path}`);
//     fileSync.queue(event, path);
//   });
// }
