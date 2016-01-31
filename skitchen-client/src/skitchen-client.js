
var Swagger = require("swagger-client");
var schema = require("skitchen-schema");

// Override the schema with our provided endpoint
schema.host = "localhost";
schema.schemes = ["http"];

class Resource {
  constructor ({swaggerResource}) {
    this.resource = swaggerResource;
  }

  find(...args) {
    return this.resource.find(...args);
  }

  findOne(...args) {
    return this.resource.findOne(...args);
  }

  insert(...args) {
    return this.resource.insert(...args);
  }

  update(...args) {
    return this.resource.update(...args);
  }

  delete(...args) {
    return this.resource.delete(...args);
  }
}

var client = new Swagger({
  spec: schema,
  usePromise: true
})

.then(function(client) {
  client.broadcasts.find().then(function() {
    // console.log(arguments);
  })
  .catch(function() {
    // console.log("eror", arguments);
  });
})

.catch(function(err) {
  // console.log("error", err);
});
