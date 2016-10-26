
import fs from "fs";
import {resolve, basename} from "path";
import yaml from "js-yaml";

const src = resolve(__dirname, "src");
const files = fs.readdirSync(resolve(__dirname, "src"));
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

const addPath = function(name, obj) {
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

files.forEach((file) => {
  const name = basename(file, ".yaml");
  const str = fs.readFileSync(resolve(src, file), "utf8");
  const parsed = yaml.safeLoad(str);
  output.definitions[name] = parsed;
  if (typeof parsed.tableName !== "undefined") {
    addPath(name, parsed);
  }
});

/*eslint-disable no-console */
console.log(JSON.stringify(output, null, 2));
