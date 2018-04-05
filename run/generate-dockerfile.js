#!/usr/bin/env node

const fs = require("fs");
const path = require("path");
const output = [];
const add = str =>
  output.push(
    ...str
      .trim()
      .split("\n")
      .map(str => str.trim())
  );

add(`
  # ==============
  # DO NOT EDIT!!!
  # ==============
  #
  # This file is generated automatically by run/generate-dockerfile.js.
  # Edit that, not this.
`);

add(`
  FROM stream.place/sp-node

  RUN npm install -g yarn lerna
  WORKDIR /app
  ENV NODE_ENV production
`);

add("# add all package.json files");

const packages = fs.readdirSync(path.resolve(__dirname, "..", "packages"));
packages.forEach(pkgName => {
  add(
    `ADD packages/${pkgName}/package.json /app/packages/${pkgName}/package.json`
  );
});

add("# build everyone's production dependencies into one big blob");

// add(`RUN yarn ${packages.map(pkgName => `/packages/${pkgName}`).join(" ")}`);
add("ADD package.json /app/package.json");
add(
  "RUN " +
    [
      // clear out dev dependencies, we don't want em
      "find . -maxdepth 3 -type f -name package.json -exec sed -i s/devDependencies/devDependenciesRemoved/ {} \\;",
      "yarn install --prod",
      "yarn cache clean"
    ].join(" && ")
);

fs.writeFileSync(
  path.resolve(__dirname, "..", "Dockerfile"),
  output.join("\n") + "\n",
  "utf8"
);
