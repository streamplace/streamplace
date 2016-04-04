/**
 * These are my extensions to node-fluent-ffmpeg, mostly making it easier to define complex
 * filtergraphs.
 */

// ["0:v", m.scale({w: 1920, h: 1080})]
//
//

import _ from "underscore";
import fluent from "fluent-ffmpeg";

import filterList from "../filterlist";

// Look ma! It's the Symbol API!
const isFilterNode = Symbol("isFilterNode");

fluent.prototype.magic = function(...args) {
  if (!this._magicFilters) {
    this._filterIdx = 0;
    this.filterLabels = {};
    this._magicFilters = [];
    this._origRun = this.run;
    this.run = function(...args) {
      this.outputOptions([
        "-filter_complex",
        this._magicFilters.join(";"),
      ]);
      return this._origRun.call(this, ...args);
    };
  }
  const inputs = [];
  let doneWithInputs = false;

  const filters = [];
  let doneWithFilters = false;

  const outputs = [];
  while (args.length > 0) {
    const arg = args.shift();

    // If it's a string...
    if (_.isString(arg)) {
      // ...and we're not done with inputs, it's an input.
      if (!doneWithInputs) {
        inputs.push(arg);
      }
      // ...and we are done with inputs, it's an output.
      else {
        doneWithInputs = true;
        doneWithFilters = true;
        outputs.push(arg);
      }
    }

    // If it's a filter...
    else if (arg[isFilterNode] === true) {
      doneWithInputs = true;
      filters.push(arg.toString());
      if (arg.label) {
        this.filterLabels[arg.label] = `Parsed_${arg.name}_${this._filterIdx}`;
      }
      this._filterIdx += 1;
    }
  }
  this._magicFilters.push([
    inputs.map(input => `[${input}]`).join(""),
    filters.join(","),
    outputs.map(output => `[${output}]`).join(""),
  ].join(""));
  return this;
};

export default class M {}

const escapeParam = function(str) {
  // We don't need no stinkin' numbers! Strings only!
  if (_.isNumber(str)) {
    str = `${str}`;
  }
  // Escape colons, they deliniate argument borders
  return str.replace(/:/g, "\\\\:");
};

class FilterNode {
  constructor(name, args) {
    this[isFilterNode] = true;
    this.name = name;
    this.positionalArgs = [];
    this.namedArgs = {};

    // Iterate over everything we were passed in.
    args.forEach((arg) => {
      // If it's a string, add it to our positional list.
      // eg M.scale("1920", "1080")
      if (_(arg).isString() || _(arg).isNumber()) {
        this.positionalArgs.push(escapeParam(arg));
      }

      // If it's an array, append it to our positional arguments.
      else if (_(arg).isArray()) {
        this.positionalArgs.push(...arg.map(escapeParam));
      }

      // If it's an object, add its k/v pairs to our named arguments.
      else if (_(arg).isObject()) {
        Object.keys(arg).forEach((k) => {
          // Special case: _label is our internal label
          if (k === "_label") {
            this.label = arg[k];
          }
          else {
            this.namedArgs[k] = escapeParam(arg[k]);
          }
        });
      }

      else {
        throw new Error("Unknown argument type: ", arg, typeof arg);
      }
    });
  }

  toString() {
    const namedOutput = Object.keys(this.namedArgs).map((name) => {
      const value = this.namedArgs[name];
      return `${name}=${value}`;
    });
    let allOutput = this.positionalArgs.concat(namedOutput).join(":");
    // Special case: we don't output anything if there are no arguments at all.
    if (allOutput.indexOf(" ") !== -1) {
      allOutput = `'${allOutput}'`;
    }
    if (allOutput === "") {
      return this.name;
    }
    else {
      return `${this.name}=${allOutput}`;
    }
  }
}

filterList.forEach(function(filterName) {
  M[filterName] = function(...args) {
    return new FilterNode(filterName, args);
  };
});
