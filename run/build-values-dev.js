
const fs = require("fs");
const yaml = require("js-yaml");
const inquirer = require("inquirer");
const path = require("path");
const winston = require("winston");
const dot = require("dot-object");

winston.cli();

const valuesPath = path.resolve(__dirname, "..", "values-dev.yaml");
const valuesExamplePath = path.resolve(__dirname, "..", "values-dev.example.yaml");

let exampleValues = yaml.safeLoad(fs.readFileSync(valuesExamplePath), "utf8");
let values;
try {
  values = fs.readFileSync(valuesPath, "utf8");
  values = yaml.safeLoad(values);
}
catch (e) {
  // If it doesn't exist, that's fine, otherwise err
  if (e.code !== "ENOENT") {
    throw e;
  }
  winston.info(`Creating ${valuesPath} with default values...`);
  values = {};
}

// Convert each thing to a dotted key/value pair
const exampleDot = dot.dot(exampleValues);
const valuesDot = dot.dot(values);
const before = JSON.stringify(valuesDot);
const prompt = [];

Object.keys(exampleDot).forEach(function(key) {
  // This section: values that we want to check every time.
  if (key === "global.rootDirectory") {
    const rootDirectory = path.resolve(__dirname, "..");
    if (rootDirectory !== valuesDot[key]) {
      winston.info(`Setting ${key}=${rootDirectory}`);
      valuesDot[key] = rootDirectory;
    }
  }
  // Everything past this point will be ok as long as it's set
  else if (valuesDot[key] !== undefined) {
    return;
  }
  else if (key === "global.domain") {
    prompt.push({
      type: "input",
      name: key,
      message: "Hey! What's your keybase username?",
      filter: function (val) {
        return `${val}.sp-dev.club`;
      }
    });
  }
  else if (key === "global.adminEmail") {
    prompt.push({
      type: "input",
      name: key,
      message: "What's the admin email for this Streamplace instance?"
    });
  }
  // All of our specially-handled prompts go here
  // And if all else fails, just copy it over.
  else {
    winston.info(`Using default value: ${key}=${exampleDot[key]}`);
    valuesDot[key] = exampleDot[key];
  }
});

const after = JSON.stringify(valuesDot);
if (before === after && prompt.length === 0) {
  // Nothing changed, we're done.
  process.exit(0);
}

inquirer.prompt(prompt)
.then((answers) => {
  // Inquirer un-dots our answers, hilariously. WE DO THE UNDOTTING HERE, INQURER
  answers = dot.dot(answers);
  Object.keys(answers).forEach((key) => {
    valuesDot[key] = answers[key];
  });
  values = dot.object(valuesDot);
  fs.writeFileSync(valuesPath, yaml.safeDump(values), "utf8");
  winston.info(`Wrote ${valuesPath}`);
})
.catch((err) => {
  winston.error(err);
  process.exit(1);
});
