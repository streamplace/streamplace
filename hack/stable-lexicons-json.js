const jss = require("json-stable-stringify");
const fs = require("fs");
const path = require("path");

const lexicons = fs.readFileSync(
  path.join(__dirname, "../lexicons.json"),
  "utf8",
);

const stableLexicons = jss(JSON.parse(lexicons), { space: "  " });

fs.writeFileSync(
  path.join(__dirname, "../lexicons.json"),
  stableLexicons + "\n",
);
