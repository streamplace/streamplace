
import url from "url";
import Swagger from "swagger-client";
import schema from "sk-schema";
import _ from "underscore";

import Resource from "./Resource";

class SKClient {
  constructor({server, log}) {
    this.shouldLog = log;
    // Override the schema with our provided endpoint
    const {protocol, host} = url.parse(server);
    schema.host = host;
    schema.schemes = [protocol.split(":")[0]];

    this.log(`SKClient initalizing for server ${schema.schemes[0]}://${schema.host}`);

    const client = new Swagger({
      spec: schema
    });

    client.buildFromSpec(schema);

    // Look at all the resources available in the freshly-parsed schema and build a Resource for
    // each one.
    client.apisArray.forEach((api) => {
      this[api.name] = new Resource({
        swaggerResource: client[api.name]
      });
    });

    client.usePromise = true;
  }

  log(...args) {
    /*eslint-disable no-console */
    if (this.shouldLog && console && console.error) {
      console.error(...args);
    }
  }
}

export default function(params) {
  return new SKClient(params);
}
