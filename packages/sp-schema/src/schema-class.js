import fs from "fs";
import yaml from "js-yaml";
import winston from "winston";
import { safeLoad as parseYaml } from "js-yaml";
import pkg from "../package.json";
import { config } from "sp-client";

const DOMAIN = config.require("DOMAIN");

winston.cli();

export class Schema {
  constructor() {
    this.schema = {
      swagger: "2.0",
      info: {
        title: "Streamplace API",
        description:
          "An API for doing awesome stuff with live streaming video.",
        version: pkg.version
      },
      plugins: {},
      host: DOMAIN,
      schemes: ["https"],
      basePath: "/api",
      produces: ["application/json"],
      consumes: ["application/json"],
      definitions: {},
      paths: {}
    };
  }

  get() {
    return this.schema;
  }

  addConfiguration({ plugin, data }) {
    if (!this.schema.plugins[plugin]) {
      this.schema.plugins[plugin] = {};
    }
    Object.keys(data).forEach(key => {
      this.schema.plugins[plugin][key] = data[key];
    });
  }

  addSchema({ plugin, name, yaml }) {
    const obj = parseYaml(yaml);
    if (!this.schema.plugins[plugin]) {
      this.schema.plugins[plugin] = {};
    }
    obj.plugin = plugin;
    this.schema.definitions[name] = obj;
    const { tableName } = obj;
    if (!tableName) {
      // Not a database-backed fella, just a definition object. That's cool.
      return;
    }
    const [first, ...rest] = tableName;
    const tableNameUpper = first.toUpperCase() + rest.join("");
    const schemaRef = { $ref: `$/definitions/${name}` };
    const responseHeaders = {
      "SP-Auth-Token": {
        description:
          "The server providing this means they are instructing you to replace your current token with this one.",
        type: "string"
      }
    };
    this.schema.paths[`/${tableName}`] = {
      get: {
        summary: `Get many ${tableName}`,
        description: `Get many ${tableName}`,
        tags: [tableName],
        operationId: `find${tableNameUpper}`,
        parameters: [
          {
            name: "filter",
            in: "query",
            description:
              "Optional JSON string. Return only documents that match.",
            required: false,
            type: "string"
          }
        ],
        responses: {
          200: {
            description: `An array of ${tableName}`,
            schema: {
              type: "array",
              items: schemaRef
            },
            headers: responseHeaders
          }
        }
      },
      post: {
        summary: `Create a ${name}`,
        tags: [tableName],
        operationId: `create${tableNameUpper}`,
        parameters: [
          {
            description: `${name} to create`,
            in: "body",
            name: "body",
            required: true,
            schema: schemaRef
          }
        ],
        responses: {
          201: {
            description: "Creation Successful",
            schema: schemaRef
          }
        },
        headers: responseHeaders
      }
    };

    this.schema.paths[`/${tableName}/{id}`] = {
      get: {
        parameters: [
          {
            name: "id",
            in: "path",
            type: "string",
            description: `id of the requested ${name}`,
            required: true
          }
        ],
        summary: `Get one ${name}`,
        description: `Get one ${name}`,
        tags: [tableName],
        operationId: `findOne${tableNameUpper}`,
        responses: {
          200: {
            description: `One retrieved ${name}`,
            schema: schemaRef
          }
        },
        headers: responseHeaders
      },

      put: {
        summary: `Modify an existing ${name}`,
        tags: [tableName],
        operationId: `update${tableNameUpper}`,
        parameters: [
          {
            description: `Content of new ${name}`,
            in: "body",
            name: "body",
            required: true,
            schema: schemaRef
          },
          {
            name: "id",
            in: "path",
            type: "string",
            description: `id of the requested ${name}`,
            required: true
          }
        ],
        responses: {
          200: {
            description: "Modification Successful",
            schema: schemaRef
          }
        },
        headers: responseHeaders
      },

      delete: {
        parameters: [
          {
            name: "id",
            in: "path",
            type: "string",
            description: `id of the requested ${name}`,
            required: true
          }
        ],
        summary: `delete a ${name}`,
        description: `delete a ${name}`,
        tags: [tableName],
        operationId: `delete${tableNameUpper}`,
        responses: {
          204: {
            description: "You did it, it's gone"
          }
        },
        headers: responseHeaders
      }
    };
  }
}
