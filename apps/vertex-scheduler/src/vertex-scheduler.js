
import SKClient from "sk-client";
import config from "sk-config";
import _ from "underscore";
import winston from "winston";

winston.cli();

import DockerPlatform from "./docker-platform";

const SK = new SKClient();

const SUCCESS_POLL = 1000;
const ERROR_POLL = 10000;
const TIMEOUT = 2 * 60 * 1000; // 2m

export default class VertexScheduler {
  constructor() {
    this.platform = new DockerPlatform();
    // setInterval(::this.reconcile, 3000);
    this.reconcile();
  }

  /**
   * Poll our platform and our processors and act appropriately.
   */
  reconcile() {
    const timeoutHandle = setTimeout(() => {
      winston.error(`Took longer than ${TIMEOUT}ms. Timeout?`);
      winston.error("I'm just going to go ahead and crash now.");
      process.exit(1);
    }, TIMEOUT);
    const cleanup = () => {
      clearTimeout(timeoutHandle);
    };
    Promise.all([
      SK.vertices.find(),
      this.platform.getVertexProcessors(),
    ])
    .then(([vertices, vertexProcessors]) => {
      return Promise.all([
        this._createNewProcessors(vertices, vertexProcessors),
        this._removeStaleProcessors(vertices, vertexProcessors),
        this._removeDuplicateProcessors(vertices, vertexProcessors),
      ]);
    })
    .then(() => {
      setTimeout(::this.reconcile, SUCCESS_POLL);
      cleanup();
    })
    .catch((err) => {
      winston.error("Error reconciling vertex schedulers.");
      winston.error(err);
      winston.error(`Scheduling next reconcile in ${ERROR_POLL}ms.`);
      setTimeout(::this.reconcile, ERROR_POLL);
      cleanup();
    });
  }

  /**
   * Create processors for new vertices.
   */
  _createNewProcessors(vertices, vertexProcessors) {
    const vertexIds = vertices.map(v => v.id);
    const verticesWithProcessor = vertexProcessors.map(vp => vp.vertexId);
    const createVertexIds = vertexIds.filter((v) => {
      return !_(verticesWithProcessor).contains(v);
    });

    if (createVertexIds.length) {
      winston.info(`Creating ${createVertexIds.length} vertex processors.`);
    }

    const promises = createVertexIds.map((vertexId) => {
      return this.platform.createVertexProcessor({vertexId})
        .then(({id}) => {
          winston.info(`Created processor ${id} for vertex ${vertexId}`);
        });
    });
    return Promise.all(promises);
  }

  /**
   * Remove processors for vertices that are gone.
   */
  _removeStaleProcessors(vertices, vertexProcessors) {
    const vertexIds = vertices.map(v => v.id);
    const removeProcessors = vertexProcessors.filter((vp) => {
      // Check if we have a processor around for a non-extant vertex.
      return !_(vertexIds).contains(vp.vertexId);
    });

    if (removeProcessors.length) {
      winston.info(`Removing ${removeProcessors.length} stale vertex processors.`);
    }

    const promises = removeProcessors.map((vp) => {
      return this.platform.removeVertexProcessor(vp.id)
        .then(() => {
          `Removed processor ${vp.id} for absent vertex ${vp.vertexId}`;
        });
    });
    return Promise.all(promises);
  }

  /**
   * Remove duplicate processors for when they're gone.
   */
  _removeDuplicateProcessors(vertices, vertexProcessors) {
    const promises = [];
    const processorsByVertex = _(vertexProcessors).groupBy("vertexId");
    _(processorsByVertex).forEach((processors, vertexId) => {
      if (processors.length < 2) {
        return;
      }
      winston.info(`Vertex ${vertexId} has ${processors.length} duplicated processors.`);
      const [keep, ...remove] = processors;
      winston.info(`Keeping ${keep.id}, removing the rest.`);
      remove.forEach((vp) => {
        const prom = this.platform.removeVertexProcessor(vp.id)
        .then(() => {
          winston.info(`Removed duplicate processor ${vp.id}`);
        });
        promises.push(prom);
      });
    });
    return Promise.all(promises);
  }
}

if (!module.parent) {
  new VertexScheduler();
}
