
import url from "url";
import Swagger from "swagger-client";
import schema from "sk-schema";
import _ from "underscore";
import IO from "socket.io-client";

import Resource from "./Resource";

class SKClient {
  constructor({server, log}) {
    this.shouldLog = log;
    // Override the schema with our provided endpoint
    const {protocol, host} = url.parse(server);
    schema.host = host;
    schema.schemes = [protocol.split(":")[0]];

    const serverURL = `${schema.schemes[0]}://${schema.host}`;
    this.log(`SKClient initalizing for server ${serverURL}`);
    this.socket = IO(server);
    this.socket.on("hello", () => {
      this.log("WebSocket init");
    });
    this.subscriptionIdx = 0;

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

  _subscriptionId() {
    const id = `${this.subscriptionIdx}`;
    this.subscriptionIdx += 1;
    return id;
  }

  _subscribe(resource, query, cb) {
    const id = this._subscriptionId();
    this.socket.emit("sub", {id, resource, query});
  }
}

export default function(params) {
  return new SKClient(params);
}
