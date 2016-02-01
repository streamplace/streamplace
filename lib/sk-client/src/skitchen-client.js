
import url from "url";
import Swagger from "swagger-client";
import schema from "skitchen-schema";

class Resource {
  constructor ({swaggerResource}) {
    this.resource = swaggerResource;
  }

  exec(action, ...args) {
    return this.resource[action](...args);
  }

  find(...args) {
    return this.exec("find", ...args);
  }

  findOne(...args) {
    return this.exec("findOne", ...args);
  }

  insert(...args) {
    return this.exec("insert", ...args);
  }

  update(...args) {
    return this.exec("update", ...args);
  }

  delete(...args) {
    return this.exec("delete", ...args);
  }
}

export default function({server}) {
  // Override the schema with our provided endpoint
  const {protocol, host} = url.parse(server);
  schema.host = host;
  schema.schemes = [protocol.split(":")[0]];

  var client = new Swagger({
    spec: schema
  });

  client.buildFromSpec(schema);

  const exports = {};

  // Look at all the resources available in the freshly-parsed schema and build a Resource for 
  // each one.
  client.apisArray.forEach((api) => {
    this[api.name] = new Resource({
      swaggerResource: client[api.name]
    });
  });

  client.usePromise = true;
}
