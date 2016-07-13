/**
 * This will be its own microservice real soon.
 */

import winston from "winston";
import config from "sk-config";
import SKClient from "sk-client";

const SK = new SKClient();

const SUCCESS_POLL = 1000;
const ERROR_POLL = 10000;
const TIMEOUT = 2 * 60 * 1000; // 2m

export default class AutosyncScheduler {
  constructor() {
    this.poll();
  }

  poll() {
    const timeoutHandle = setTimeout(() => {
      winston.error(`Took longer than ${TIMEOUT}ms. Timeout?`);
      winston.error("I'm just going to go ahead and crash now.");
      process.exit(1);
    }, TIMEOUT);
    const cleanup = () => {
      clearTimeout(timeoutHandle);
    };
    SK.vertices.find({})
    .then((vertices) => {
      const rtmpInputVertices = vertices.filter(v => v.type === "RTMPInput");
      const autosyncVertices = vertices.filter(v => v.type === "Autosync");
      return this.reconcile(rtmpInputVertices, autosyncVertices);
    })
    .then(() => {
      setTimeout(::this.poll, SUCCESS_POLL);
      cleanup();
    })
    .catch((err) => {
      winston.error("Error reconciling autosync schedulers.");
      winston.error(err);
      winston.error(`Scheduling next poll in ${ERROR_POLL}ms.`);
      setTimeout(::this.poll, ERROR_POLL);
      cleanup();
    });
  }

  reconcile(rtmpInputVertices, autosyncVertices) {
    return Promise.all([
      this._reconcileStaleVertices(rtmpInputVertices, autosyncVertices),
      this._reconcileNewVertices(rtmpInputVertices, autosyncVertices),
    ]);
  }

  _reconcileStaleVertices(rtmpInputVertices, autosyncVertices) {
    return Promise.all(autosyncVertices.map((autosyncVertex) => {
      const [inputVertex] = rtmpInputVertices.map((v) => {
        return v.id === autosyncVertex.params.inputVertexId;
      });

      // Is our input vertex completely gone? Okay, no need to continue existing.
      if (!inputVertex) {
        winston.info(`Deleting stale autosync vertex ${autosyncVertex.title}`);
        return SK.vertices.delete(autosyncVertex.id);
      }

      // Does our input vertex a sync timestamp already? Neat, it can go die.
      if (inputVertex.autosyncTimestamp) {
        winston.info(`Deleting stale autosync vertex ${autosyncVertex.title}`);
        return SK.vertices.delete(autosyncVertex.id);
      }

      // Okay, you still get to exist.
      return Promise.resolve();
    }));
  }

  _reconcileNewVertices(rtmpInputVertices, autosyncVertices) {
    return Promise.all(rtmpInputVertices.map((inputVertex) => {

      // Does it already have a timestamp? Great! Nothing for us to do.
      if (inputVertex.autosyncTimestamp !== undefined) {
        return Promise.resolve();
      }

      const syncVertices = autosyncVertices.filter(v => v.params.inputVertexId === inputVertex.id);
      // Does it already have an autosync vertex? Cool, nothing for us to do.
      if (syncVertices.length === 1) {
        return Promise.resolve();
      }
      if (syncVertices.length > 1) {
        const [first, ...deleteThese] = syncVertices;
        winston.error(`Found more than one autosync vertex for ${inputVertex.id}, deleting all but ${first.id}.`);
        return Promise.all(deleteThese.map((vertex) => {
          return SK.vertices.delete(vertex.id);
        }));
      }

      // Okay, new input, needs an autosync. Let's go.
      winston.info(`Creating autosync vertex for ${inputVertex.title}`);
      return SK.vertices.create({
        kind: "vertex",
        type: "Autosync",
        title: `${inputVertex.title} Sync`,
        image: config.require("VERTEX_PROCESSOR_IMAGE"),
        broadcastId: inputVertex.broadcastId,
        status: "INACTIVE",
        inputs: [{
          name: "default",
          sockets: [{
            type: "audio"
          }]
        }],
        outputs: [],
        params: {
          inputVertexId: inputVertex.id
        }
      })
      .then((newVertex) => {
        return SK.arcs.create({
          kind: "arc",
          broadcastId: inputVertex.broadcastId,
          delay: 0,
          from: {
            ioName: "default",
            vertexId: inputVertex.id
          },
          to: {
            ioName: "default",
            vertexId: newVertex.id
          }
        });
      });
    }));
  }
}

if (!module.parent) {
  new AutosyncScheduler();
}
