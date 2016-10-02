
import url from "url";
import Swagger from "swagger-client";
import schema from "sk-schema";
import _ from "underscore";
import IO from "socket.io-client";
import config from "sk-config";
import EE from "wolfy87-eventemitter";

import Resource from "./Resource";

const PUBLIC_AUTH_MODE = config.require("PUBLIC_AUTH_MODE");
const API_SERVER_URL = config.optional("API_SERVER_URL");
const PUBLIC_API_SERVER_URL = config.optional("PUBLIC_API_SERVER_URL");

let isNode = true;
if (typeof window === "object") {
  isNode = false;
}

let tokenGenerator;
if (isNode) {
  // Someone teach me a better wneway to have node do something but not webpack.
  /*eslint-disable no-eval */
  const TokenGenerator = eval("require('./TokenGenerator')").default;
  tokenGenerator = new TokenGenerator();
}

export default class SKClient extends EE {
  constructor({server, log, start, token} = {}) {
    super();
    this.shouldLog = log;
    this.server = server;
    this.token = token;
    if (start !== false) {
      this.connect({server, log});
    }
  }

  connect({server, log, token} = {}) {
    this.connected = false;
    this.shouldLog = log;
    this.token = token || this.token;
    if (!server) {
      server = this.server;
    }
    if (!server) {
      server = API_SERVER_URL;
    }
    if (!server) {
      server = PUBLIC_API_SERVER_URL;
    }
    if (!server) {
      throw new Error("Tried to instantate SKClient with no server provided explicitly. API_SERVER_URL and PUBLIC_API_SERVER_URL are also both unset.");
    }
    this.server = server;
    // Override the schema with our provided endpoint
    const {protocol, host} = url.parse(server);
    schema.host = host;
    schema.schemes = [protocol.split(":")[0]];

    // Generate a token if we can and we don't have one.
    if (!this.token && tokenGenerator) {
      this.token = tokenGenerator.generate();
      // Generate new tokens when we're halfway to token expiration.
      setInterval(() => {
        this.token = tokenGenerator.generate();
      }, Math.floor(tokenGenerator.expiry / 2));
    }

    // Set up HTTP connection based on Swagger schema
    const serverURL = `${schema.schemes[0]}://${schema.host}`;
    this.log(`SKClient initalizing for server ${serverURL}`);

    const authorizations = {};
    if (this.token) {
      authorizations.sk = new Swagger.ApiKeyAuthorization("SK-Auth-Token", this.token, "header");
    }

    const client = new Swagger({
      spec: schema,
      authorizations
    });
    client.buildFromSpec(schema);

    let socketServer = server;
    if (this.token) {
      socketServer = `${server}?token=${this.token}`;
    }

    // Set up websocket connection
    this.socket = IO(socketServer, {
      transports: ["websocket"]
    });
    this.socket.on("hello", () => {
      this.connected = true;
      _(this.activeSubscriptions).each( ({resource, query, cb}, subId) => {
        this.log("sub", resource, query);
        this.socket.emit("sub", {subId, resource, query});
      });
    });


    this.socket.on("data", ({tableName, id, doc}) => {
      if (!this[tableName]) {
        throw new Error(`Got data event for ${tableName}, but I dunno what that is.`);
      }
      this[tableName]._data(id, doc);
    });

    this.socket.on("suback", ({subId}) => {
      this.activeSubscriptions[subId] && this.activeSubscriptions[subId].cb();
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

    this.socket.on("error", ::this._handleError);

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

  _handleError(err) {
    err = JSON.parse(err);
    this.log(err);
    this.emit("error", err);
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
    const subId = `${this.subscriptionIdx}`;
    this.subscriptionIdx += 1;
    return subId;
  }

  _subscribe(resource, query, cb) {
    const subId = this._subscriptionId();
    this.activeSubscriptions[subId] = {resource, query, cb};
    if (this.connected) {
      this.socket.emit("sub", {subId, resource, query});
    }
    return {
      stop: () => {
        this.socket.emit("unsub", {subId});
        delete this.activeSubscriptions[subId];
      }
    };
  }
}
