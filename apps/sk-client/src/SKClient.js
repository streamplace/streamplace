
import url from "url";
import Swagger from "swagger-client";
import schema from "sk-schema";
import _ from "underscore";
import IO from "socket.io-client";
import config from "sk-config";

import Resource from "./Resource";

class SKClient {
  constructor({server, log} = {}) {
    this.connected = false;
    this.shouldLog = log;
    if (!server) {
      server = config.require("API_SERVER_URL");
    }
    // test
    // Override the schema with our provided endpoint
    const {protocol, host} = url.parse(server);
    schema.host = host;
    schema.schemes = [protocol.split(":")[0]];

    // Set up HTTP connection based on Swagger schema
    const serverURL = `${schema.schemes[0]}://${schema.host}`;
    this.log(`SKClient initalizing for server ${serverURL}`);

    const client = new Swagger({
      spec: schema
    });
    client.buildFromSpec(schema);

    // Set up websocket connection
    this.socket = IO(server);
    this.socket.on("hello", () => {
      this.connected = true;
      _(this.activeSubscriptions).each( ({resource, query, cb}, id) => {
        this.log("sub", resource, query);
        this.socket.emit("sub", {id, resource, query});
      });
    });
    this.subscriptionIdx = 0;
    this.activeSubscriptions = {};

    const handler = this._handleMessage.bind(this);
    ([
      "hello",
      "suback",
      "created",
      "updated",
      "deleted",
      "connect",
      "connect_error",
      "connect_timeout",
      "reconnect",
      "reconnect_attempt",
      "reconnecting",
      "reconnect_error",
      "reconnect_failed",
    ]).forEach((msg) => {
      this.socket.on(msg, this._handleMessage(msg));
    });

    // Look at all the resources available in the freshly-parsed schema and build a Resource for
    // each one.
    client.apisArray.forEach((api) => {
      this[api.name] = new Resource({
        SK: this,
        swaggerResource: client[api.name]
      });
    });

    client.usePromise = true;
  }

  _handleMessage(eventName) {
    return (evt = {}) => {
      // this.log(`socket ${eventName} ${JSON.stringify({subId: evt.subId})}`);
      if (evt.subId && this.activeSubscriptions[evt.subId]) {
        this.activeSubscriptions[evt.subId].cb(eventName, evt);
      }
    };
  }

  log(...args) {
    /*eslint-disable no-console */
    if (this.shouldLog && console && console.info) {
      console.info(...args);
    }
  }

  _subscriptionId() {
    const id = `${this.subscriptionIdx}`;
    this.subscriptionIdx += 1;
    return id;
  }

  _subscribe(resource, query, cb) {
    const id = this._subscriptionId();
    this.activeSubscriptions[id] = {resource, query, cb};
    if (this.connected) {
      this.socket.emit("sub", {id, resource, query});
    }
    return {
      stop: () => {
        this.socket.emit("unsub", {subId: id});
        delete this.activeSubscriptions[id];
      }
    };
  }
}

export default function(params) {
  return new SKClient(params);
}
