
// Threw this together real real fast when I needed it. Sorry. Could probably use a restructuring.
/*eslint-disable no-console */

import winston from "winston";
import program from "commander";

import ENV from "./env";
import SK from "./sk";

// Do a thing then exit, bypassing the auto-include of Commander
const go = function(action) {
  return function(...args) {
    try {
      action(...args);
    }
    catch (e) {
      console.error(`Unexpected error: ${e.message}`);
      console.error(e.stack);
      process.exit(1);
    }
  };
};

const aliases = {
  "broadcast": "broadcasts",
  "vertex": "vertices",
  "arc": "arcs",
};

const getAlias = function(str) {
  if (aliases[str]) {
    return aliases[str];
  }
  return str;
};

const fatal = function(...args) {
  console.log("fatal error");
  console.error(...args);
  process.exit(1);
};

const serverError = function(err) {
  console.error(`Server Error (${err.status}): ${err.message}`);
  process.exit(1);
};

const spitJSON = function(str) {
  let output;
  try {
    output = JSON.stringify(str, null, 4);
  }
  catch (e) {
    console.error("Error decoding JSON");
    throw e;
  }
  console.log(output);
};

const parseJSON = function(str) {
  let parsed;
  try {
    parsed = JSON.parse(str);
  }
  catch (e) {
    fatal("Error parsing JSON: " + e.message);
  }
  return parsed;
};

program
  .version("0.0.0")
  .usage("[action] [resource]")

  .action(function(cmd, resource, ...args) {

    resource = getAlias(resource);

    if (typeof resource !== "string") {
      fatal("Unknown resource: " + resource);
    }

    if (cmd === "get") {
      const [query] = args;
      if (typeof query !== "string") {
        fatal("Need exactly two string arguments, please");
      }
      SK[resource].findOne(query).then(spitJSON).catch(serverError);
    }

    else if (cmd === "create") {
      let [doc] = args;
      doc = parseJSON(doc);
      SK[resource].create(doc).then(spitJSON).catch(serverError);
    }

    else if (cmd === "update") {
      let [id, doc] = args;
      if (typeof id !== "string") {
        fatal("Need exactly two string arguments, please");
      }
      doc = parseJSON(doc);
      SK[resource].create(doc).then(spitJSON).catch(serverError);
    }

    else if (cmd === "delete") {
      let [id] = args;
      if (typeof id !== "string") {
        fatal("Need exactly two string arguments, please");
      }
      SK[resource].delete(id).catch(serverError).then(function() {
        console.log("deleted");
      });
    }

    else {
      fatal("Unknown command: " + cmd);
    }
  })

  .parse(process.argv);
// Main loop. Watch for broadcasts that I'm supposed to manage.
