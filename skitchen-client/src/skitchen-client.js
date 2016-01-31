
import Swagger from "swagger-client";
import schema from "skitchen-schema";

// Override the schema with our provided endpoint
schema.host = "localhost";
schema.schemes = ["http"];

class Resource {
  constructor ({swaggerResource}) {
    this.resource = swaggerResource;
  }

  exec(action, ...args) {
    return new Promise((resolve, reject) => {
      this.resource[action](...args, resolve, reject);
    });
  }

  find(...args) {
    return this.exec('find', ...args);
  }

  findOne(...args) {
    return this.exec('findOne', ...args);
  }

  insert(...args) {
    return this.exec('insert', ...args);
  }

  update(...args) {
    return this.exec('update', ...args);
  }

  delete(...args) {
    return this.exec('delete', ...args);
  }
}

var client = new Swagger({
  spec: schema
});

client.buildFromSpec(schema);

const exports = {};

// Look at all the resources available in the freshly-parsed schema and build a Resource for each 
// one.
client.apisArray.forEach(function(api) {
  exports[api.name] = new Resource({
    swaggerResource: client[api.name]
  });
  console.log('setting ' + api.name);
});

export default exports;
