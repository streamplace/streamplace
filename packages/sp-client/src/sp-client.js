import url from "url";
import Swagger from "swagger-client";
import _ from "underscore";
import IO from "socket.io-client";
import EE from "wolfy87-eventemitter";
import jwtDecode from "jwt-decode";
import request from "superagent";
import config from "sp-configuration";

import Resource from "./Resource";

const API_SERVER_URL = config.optional("API_SERVER_URL");
const PUBLIC_API_SERVER_URL = config.optional("PUBLIC_API_SERVER_URL");
const DOMAIN = config.optional("DOMAIN");
const SCHEMA_URL = config.optional("SCHEMA_URL");

const IMPORTANT_EVENTS = [
  "hello",
  "connect",
  "connect_error",
  "connect_timeout",
  "reconnect",
  "reconnect_attempt",
  "reconnecting",
  "reconnect_error",
  "reconnect_failed"
];

let isNode = true;
if (typeof window === "object") {
  isNode = false;
}

/**
 * We use HTTP error codes to know how to handle our errors usually, so here's a little helper for
 * that.
 */
const apiError = function(code, message) {
  const err = new Error(message);
  err.status = code;
  return err;
};

export class SPClient extends EE {
  constructor({ server, log, token, app } = {}) {
    super();
    if (isNode) {
      // Someone teach me a better wneway to have node do something but not webpack.
      /*eslint-disable no-eval */
      const TokenGenerator = eval("require('./TokenGenerator')").default;
      this.tokenGenerator = new TokenGenerator({ app: this.app });
    }
    this.app = app || "spclient";
    this.shouldLog = true;
    this.server = server;
    this.token = token;
  }

  connect({ server, log, token } = {}) {
    this.connected = false;
    this.shouldLog = true;
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
    if (!server && DOMAIN) {
      server = `https://${DOMAIN}`;
    }
    if (!server) {
      throw new Error(
        "Tried to instantate SPClient with no server provided explicitly. API_SERVER_URL and PUBLIC_API_SERVER_URL are also both unset."
      );
    }
    this.server = server;
    // Kill trailing slash if present
    if (this.server.slice(-1) === "/") {
      this.server = this.server.slice(0, this.server.length - 1);
    }
    const schemaUrl = SCHEMA_URL || this.server;
    return request(`${schemaUrl}/schema.json`)
      .then(res => {
        const schema = res.body;
        // If we have API_SERVER_URL, we're in a cluster, and should trust it over the schema
        if (API_SERVER_URL) {
          let { protocol, host, path } = url.parse(API_SERVER_URL);
          protocol = protocol.slice(0, -1);
          schema.host = host;
          schema.schemes = [protocol];
          schema.basePath = path;
        }
        this.schema = schema;

        // Generate a token if we can and we don't have one.
        if (!this.token && this.tokenGenerator) {
          this.token = this.tokenGenerator.generate();
        }
        if (!this.token) {
          return Promise.reject(apiError(401, "No token provided"));
        }

        this.client = new Swagger({
          spec: this.schema
        });
        this.client.buildFromSpec(this.schema);

        this.newToken(this.token);

        // Look at all the resources available in the freshly-parsed schema and build a Resource for
        // each one.
        this.client.apisArray.forEach(api => {
          this[api.name] = new Resource({
            SK: this,
            swaggerResource: this.client[api.name]
          });
        });

        this.client.usePromise = true;

        return this.getUser();
      })
      .then(user => {
        // Set up HTTP connection based on Swagger schema
        const serverURL = `${this.schema.schemes[0]}://${this.schema.host}`;
        this.log(`SPClient initalizing for server ${serverURL}`);

        this.user = user;
        let socketServer = `${serverURL}/`;
        if (this.token) {
          socketServer = `${socketServer}?token=${this.token}`;
        }

        let socketPath = this.schema.basePath + "/socket/socket.io";

        // Set up websocket connection
        this.socket = IO(socketServer, {
          path: socketPath,
          transports: ["websocket"]
        });
        this.socket.on("hello", () => {
          this.connected = true;
          _(this.activeSubscriptions).each(({ resource, query, cb }, subId) => {
            this.log("sub", resource, query);
            this.socket.emit("sub", { subId, resource, query });
          });
        });

        this.socket.on("data", ({ tableName, id, doc }) => {
          if (!this[tableName]) {
            throw new Error(
              `Got data event for ${tableName}, but I dunno what that is.`
            );
          }
          this[tableName]._data(id, doc);
        });

        this.socket.on("suback", ({ subId }) => {
          this.activeSubscriptions[subId] &&
            this.activeSubscriptions[subId].cb();
        });
        this.subscriptionIdx = 0;
        this.activeSubscriptions = {};

        const handler = this._handleMessage.bind(this);
        [
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
          "reconnect_failed"
        ].forEach(msg => {
          this.socket.on(msg, this._handleMessage(msg));
        });

        this.socket.on("error", ::this._handleError);

        this.emit("ready");
        return this.user;
      })
      .catch(err => {
        this.log(err);
        throw err;
      });
  }

  disconnect() {
    this.socket.close();
  }

  newToken(token) {
    this.token = token;
    this.client.clientAuthorizations.add(
      "SP",
      new Swagger.ApiKeyAuthorization("sp-auth-token", token, "header")
    );
  }

  getUser() {
    const beforeToken = this.token;
    const decoded = jwtDecode(this.token);
    return this.users.find({ identity: decoded.sub }).then(([user]) => {
      // If we didn't find one and our token changed, the server gently let us know we're actually
      // someone else. Try again!
      if (!user) {
        if (this.token !== beforeToken) {
          return this.getUser();
        }
        throw new Error(`Cannot find user with identity="${decoded.sub}"`);
      }
      return user;
    });
  }

  _handleError(err) {
    try {
      err = JSON.parse(err);
    } catch (e) {
      err = new Error(err);
    }
    this.log(err);
    this.emit("error", err);
  }

  _handleMessage(eventName) {
    return (evt = {}) => {
      if (IMPORTANT_EVENTS.includes(eventName)) {
        this.log(`socket ${eventName} ${JSON.stringify({ subId: evt.subId })}`);
      }
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

  // Couple of shortcuts. Might be complex client-side reporting in the future, who knows?
  info(...args) {
    console.info(...args);
  }

  error(...args) {
    console.info(...args);
  }

  _subscriptionId() {
    const subId = `${this.subscriptionIdx}`;
    this.subscriptionIdx += 1;
    return subId;
  }

  _subscribe(resource, query, cb) {
    const subId = this._subscriptionId();
    this.activeSubscriptions[subId] = { resource, query, cb };
    if (this.connected) {
      this.socket.emit("sub", { subId, resource, query });
    }
    return {
      stop: () => {
        this.socket.emit("unsub", { subId });
        delete this.activeSubscriptions[subId];
      }
    };
  }
}

const SP = new SPClient({});

if (!isNode) {
  window.SP = SP;
}

export default SP;
