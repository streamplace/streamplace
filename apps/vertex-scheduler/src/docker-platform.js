
/**
 * This is a scheduler platform that wraps Docker, via dockerode. The idea here is that this same
 * API can be implemented as a Kubernetes platform when the time comes for that.
 */

import Docker from "dockerode";

const STOP_GRACE_PERIOD = 5000;

export default class DockerPlatform {
  constructor() {
    this.docker = new Docker();
  }

  /**
   * Get all vertex processors currently active in this environment. Must return objects
   * containing {id, vertexId}. Ids are opaque to the rest of the application, but can be used to
   * make other calls regarding the container.
   */
  getVertexProcessors() {
    return new Promise((resolve, reject) => {
      this.docker.listContainers(function (err, containers) {
        if (err) {
          return reject(err);
        }
        const relevantContainers = containers
          .filter((c) => {
            return c.Labels["kitchen.stream.vertex-processor"] === "true";
          })
          .map((c) => {
            return {
              id: c.Id,
              vertexId: c.Labels["kitchen.stream.vertex-id"],
            };
          });
        resolve(relevantContainers);
      });
    });
  }

  _getContainer(vertex, environment) {
    return {
      Image: vertex.image,
      Labels: {
        "kitchen.stream.vertex-processor": "true",
        "kitchen.stream.vertex-id": vertex.id,
      },
      Env: Object.keys(environment).map((key) => {
        const value = environment[key];
        return `SK_${key}=${value}`;
      }),
      HostConfig: {
        NetworkMode: "host"
      },
    };
  }

  createVertexProcessor(vertex, environment) {
    return new Promise((resolve, reject) => {
      const tmpl = this._getContainer(vertex, environment);
      this.docker.createContainer(tmpl, (err, container) => {
        if (err) {
          return reject(err);
        }
        container.attach({stream: true, stdout: true, stderr: true}, (err, stream) => {
          if (err) {
            reject(err);
          }
          stream.pipe(process.stdout);
          container.start((err) => {
            if (err) {
              return reject(err);
            }
            resolve({id: container.id});
          });
        });
      });
    });
  }

  removeVertexProcessor(id) {
    return new Promise((resolve, reject) => {
      const container = this.docker.getContainer(id);
      container.stop((err, data) => {
        if (err) {
          return reject(err);
        }
        setTimeout(() => {
          container.remove((err, data) => {
            if (err) {
              return reject(err);
            }
            resolve();
          });
        }, STOP_GRACE_PERIOD);
      });
    });
  }
}
