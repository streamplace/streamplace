
import url from "url";
import Swagger from "swagger-client";
import schema from "sk-schema";

class Resource {
  constructor ({swaggerResource}) {
    this.resource = swaggerResource;
  }

  onSuccess(response) {
    return response.obj;
  }

  onError(err) {
    // Regular server error, we can dig it.
    if (err.status && err.obj && err.obj.message) {
      throw {
        status: err.status,
        message: err.obj.message,
        code: err.obj.code || "UNKNOWN_ERROR"
      };
    }
    // Hmm. Unexpected error. Possibly connectivity problems?
    else {
      let message = "Unknown Error";
      if (err instanceof Error) {
        message = err.message;
      }
      throw new Error({
        status: -1,
        message: message,
        code: "UNKNOWN_ERROR"
      });
    }
  }

  /**
   * This will take a selector someday.
   * @return {[type]} [description]
   */
  find() {
    return this.resource.find()
      .then(this.onSuccess)
      .catch(this.onError);
  }

  findOne(id) {
    return this.resource.findOne({id})
      .then(this.onSuccess)
      .catch(this.onError);
  }

  create(doc) {
    return this.resource.create({body: doc})
      .then(this.onSuccess)
      .catch(this.onError);
  }

  update(id, fields) {
    return this.resource.update({id: id, body: fields})
      .then(this.onSuccess)
      .catch(this.onError);
  }

  delete(id) {
    return this.resource.delete(id)
      .then(this.onSuccess)
      .catch(this.onError);
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
