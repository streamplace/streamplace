
import fs from "fs";
import {resolve, basename} from "path";
import yaml from "js-yaml";
import config from "sk-config";
import finder from "find-package-json";
import chokidar from "chokidar";
import winston from "winston";
import _ from "underscore";

winston.cli();

const plugins = config.require("PLUGINS").split(/\s+/).filter(p => p !== "");

const addPath = function(output, name, obj) {
  const {tableName} = obj;
  const [first, ...rest] = tableName;
  const tableNameUpper = first.toUpperCase() + rest.join("");
  const paths = output.paths;
  const schemaRef = {"$ref": `$/definitions/${name}`};
  paths[`/${tableName}`] = {
    get: {
      summary: `Get many ${tableName}`,
      description: `Get many ${tableName}`,
      tags: [tableName],
      operationId: `find${tableNameUpper}`,
      parameters: [{
        name: "filter",
        in: "query",
        description: "Optional JSON string. Return only documents that match.",
        required: false,
        type: "string",
      }],
      responses: {
        200: {
          description: `An array of ${tableName}`,
          schema: {
            type: "array",
            items: schemaRef
          }
        }
      }
    },
    post: {
      summary: `Create a ${name}`,
      tags: [tableName],
      operationId: `create${tableNameUpper}`,
      parameters: [{
        description: `${name} to create`,
        in: "body",
        name: "body",
        required: true,
        schema: schemaRef,
      }],
      responses: {
        201: {
          description: "Creation Successful",
          schema: schemaRef
        }
      }
    }
  };

  paths[`/${tableName}/{id}`] = {
    get: {
      parameters: [{
        name: "id",
        in: "path",
        type: "string",
        description: `id of the requested ${name}`,
        required: true
      }],
      summary: `Get one ${name}`,
      description: `Get one ${name}`,
      tags: [tableName],
      operationId: `findOne${tableNameUpper}`,
      responses: {
        200: {
          description: `One retrieved ${name}`,
          schema: schemaRef
        }
      }
    },

    put: {
      summary: `Modify an existing ${name}`,
      tags: [tableName],
      operationId: `update${tableNameUpper}`,
      parameters: [{
        description: `Content of new ${name}`,
        in: "body",
        name: "body",
        required: true,
        schema: schemaRef
      }, {
        name: "id",
        in: "path",
        type: "string",
        description: `id of the requested ${name}`,
        required: true
      }],
      responses: {
        200: {
          description: "Modification Successful",
          schema: schemaRef
        }
      }
    },

    delete: {
      parameters: [{
        name: "id",
        in: "path",
        type: "string",
        description: `id of the requested ${name}`,
        required: true
      }],
      summary: `delete a ${name}`,
      description: `delete a ${name}`,
      tags: [tableName],
      operationId: `delete${tableNameUpper}`,
      responses: {
        204: {
          description: "You did it, it's gone"
        }
      }
    },
  };
};

const watchDirs = [];

const regenerate = function() {
  const output = {
    swagger: "2.0",
    info: {
      title: "Stream Kitchen API",
      description: "An API for doing awesome stuff with live streaming video.",
      version: "0.0.0-alpha1"
    },
    host: "api.stream.kitchen",
    schemes: ["https"],
    basePath: "/v0",
    produces: ["application/json"],
    consumes: ["application/json"],
    definitions: {},
    paths: {}
  };

  plugins.forEach((plugin) => {
    const f = finder(require.resolve(plugin));
    const value = f.next().value;
    if (!value) {
      throw new Error(`Couldn't find package.json for ${plugin}`);
    }
    const schemaPath = resolve(value.__path, "..", "schema");
    watchDirs.push(schemaPath);
    const files = fs.readdirSync(schemaPath);
    files.forEach((file) => {
      const name = basename(file, ".yaml");
      let str;
      let parsed;
      let err;
      try {
        str = fs.readFileSync(resolve(schemaPath, file), "utf8");
        parsed = yaml.safeLoad(str);
      }
      catch (e) {
        err = e;
      }
      if (err || !parsed) {
        err && winston.error(err);
        winston.error(`Error parsing ${file}, ignoring for now.`);
        return;
      }
      output.definitions[name] = parsed;
      if (typeof parsed.tableName !== "undefined") {
        addPath(output, name, parsed);
      }
    });
  });
  const outPath = resolve(__dirname, "..", "dist", "schema.json");
  fs.writeFileSync(outPath, JSON.stringify(output, null, 2), "utf8");
  winston.info(`Wrote ${outPath}`);
};

regenerate();

if (!module.parent && process.argv[2] && process.argv[2] === "--watch") {
  watchDirs.forEach((dir) => {
    winston.info(`Watching ${dir}`);
    const watcher = chokidar.watch(dir);
    const regen = _.debounce(regenerate, 100);
    watcher.on("ready", () => {
      watcher
        .on("add", (file) => {
          winston.info(`${file} added`);
          regen();
        })
        .on("change", (file) => {
          winston.info(`${file} changed`);
          regen();
        })
        .on("unlink", (file) => {
          winston.info(`${file} deleted`);
          regen();
        });
    });
  });
}

