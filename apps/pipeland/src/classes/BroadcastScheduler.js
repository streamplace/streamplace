/**
 * This will be its own microservice real soon.
 */

import winston from "winston";
import _ from "underscore";

import SK from "../sk";

export default class BroadcastScheduler {
  constructor() {
    this.ready = false;
    this.broadcasts = [];
    this.scenes = [];
    this.vertices = [];

    this.broadcastHandle = SK.broadcasts.watch({})
    .on("data", (broadcasts) => {
      this.broadcasts = broadcasts;
      this.reconcile();
    })
    .catch(::this.error);

    this.sceneHandle = SK.scenes.watch({})
    .on("data", (scenes) => {
      this.scenes = scenes;
      this.reconcile();
    })
    .catch(::this.error);

    this.vertexHandle = SK.vertices.watch({})
    .on("data", (vertices) => {
      this.vertices = vertices;
      this.reconcile();
    })
    .catch(::this.error);

    this.arcHandle = SK.arcs.watch({})
    .on("data", (arcs) => {
      this.arcs = arcs;
      this.reconcile();
    })
    .catch(::this.error);

    Promise.all([this.vertexHandle, this.sceneHandle, this.arcHandle, this.broadcastHandle])
    .then(() => {
      this.ready = true;
      this.reconcile = _(::this.reconcile).throttle(500);
      this.reconcile();
    })
    .catch(::this.error);


  }

  cleanup() {
    this.broadcastHandle.stop();
    this.sceneHandle.stop();
    this.vertexHandle.stop();
  }

  reconcile() {
    if (!this.ready) {
      return;
    }
    this.broadcasts.forEach(::this.reconcileBroadcast);
  }

  /**
   * This thing is designed to be all schedule-y and idempotent and all that fun stuff. Here's a
   * broadcast, make sure the API ecosystem reflects everything you want to be true about that
   * broadcast. Make no assumptions about whether it's live or whatever.
   */
  reconcileBroadcast(broadcast) {
    const scenes = _(this.scenes).filter({broadcastId: broadcast.id});
    const vertices = _(this.vertices).filter({broadcastId: broadcast.id});
    if (this.scenes.length === 0) {
      // "Unmanaged" broadcast for now. We don't care.
      return;
    }
    if (!(broadcast.enabled === true)) {
      this.debug(`${broadcast.id} is inactive.`);
      vertices.forEach((vertex) => {
        this.debug(`Deleting vertex ${vertex.id}, its broadcast is inactive.`);
        SK.vertices.delete(vertex.id).catch(::this.error);
      });
      return;
    }
    // Make a list of all required inputs plz
    const inputIds = new Set();
    scenes.forEach((scene) => {
      scene.regions.forEach((region) => {
        inputIds.add(region.inputId);
      });
    });

    // Okay, active broadcast. Let's see what we need to make.
    const creationQueue = [];
    inputIds.forEach((inputId) => {
      let inputVertices = vertices.filter((vertex) => {
        return vertex.params.inputId === inputId;
      });
      if (inputVertices.length === 0) {
        this.debug(`Didn't find a vertex for input ${inputId}, creating one.`);
        creationQueue.push({
          kind: "vertex",
          type: "RTMPInput",
          broadcastId: broadcast.id,
          title: `Input ${inputId}`,
          status: "INACTIVE",
          inputs: [],
          outputs: [{
            name: "default",
            sockets: [{
              type: "video"
            }, {
              type: "audio"
            }]
          }],
          params: {
            inputId: inputId
          }
        });
      }
      else if (inputVertices.length === 1) {
        this.debug(`Found vertex ${inputVertices[0].title} for input ${inputId}`);
      }
      else if (inputVertices.length > 1) {
        this.error(`Found more than one input vertex for input ${inputId}`);
      }
    });

    creationQueue.forEach((newVertex) => {
      SK.vertices.create(newVertex).catch(::this.error);
    });

    const compositeVertices = _(this.vertices).filter({type: "Composite"});
    let compositeVertex;
    if (compositeVertices.length < 1) {
      this.debug(`Creating Composite Vertex for broadcast ${broadcast.title}`);
      const newVertex = {
        kind: "vertex",
        type: "Composite",
        broadcastId: broadcast.id,
        title: "Compositor",
        status: "INACTIVE",
        inputs: ([...inputIds]).map((inputId) => {
          return {
            name: inputId,
            sockets: [{
              type: "video"
            }, {
              type: "audio"
            }]
          };
        }),
        outputs: [{
          name: "default",
          sockets: [{
            type: "video"
          }, {
            type: "audio"
          }]
        }],
        params: {}
      };
      SK.vertices.create(newVertex).catch(::this.error);
    }

    else if (compositeVertices.length === 1) {
      this.debug(`Found composite vertex ${compositeVertices[0].title} for broadcast ${broadcast.title}`);
      const compositeVertex = compositeVertices[0];
      compositeVertex.inputs.forEach((input) => {
        let inputVertex = vertices.filter((vertex) => {
          return vertex.params.inputId === input.name;
        })[0];
        if (!inputVertex) {
          this.debug(`Couldn't find an input vertex for ${input.name}, waiting to create an arc.`);
          return;
        }
        let arcs = this.arcs.filter((arc) => {
          return arc.from.vertexId === inputVertex.id && arc.to.vertexId === compositeVertex.id && arc.to.ioName === input.name;
        });
        if (arcs.length === 0) {
          this.debug(`Creating arc from ${inputVertex.id} to ${compositeVertex.id}`);
          SK.arcs.create({
            kind: "arc",
            broadcastId: broadcast.id,
            delay: 0,
            from: {
              ioName: "default",
              vertexId: inputVertex.id,
            },
            to: {
              ioName: input.name,
              vertexId: compositeVertex.id
            }
          }).catch(::this.error);
        }
      });
    }
    else if (compositeVertices.length > 1) {
      this.error(`Found more than one composite vertex for broadcast ${broadcast.title}`);
    }
  }

  debug(msg, ...args) {
    winston.debug(`[BroadcastScheduler] ${msg}`, ...args);
  }

  info(msg, ...args) {
    winston.info(`[BroadcastScheduler] ${msg}`, ...args);
  }

  error(msg, ...args) {
    winston.error("[BroadcastScheduler]", msg, ...args);
  }
}
