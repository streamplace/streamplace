
/**
 * Here's how it is. InputVertices send data to a CompositeVertex. Autosync stuff writes our best
 * guess as to the start time of that stream to that inputvertex. Now the CompositeVertex needs to
 * rewrite the PTS of that stream prior to passing it to FFMpeg. That's our job -- given an input
 * name and type, and a CompositeVertex id, figure out which InputVertex is sending us data and
 * rewrite the stream accordingly. Don't output anything until you know what the sync is!
 */

import MpegMungerStream from "mpeg-munger";
import winston from "winston";

import SK from "../sk";
import {SERVER_START_TIME, TIME_BASE} from "../constants";

export default class SyncInputStream extends MpegMungerStream {
  constructor(params) {
    super(params);
    const {compositeVertexId, inputName} = params;
    if (!compositeVertexId || !inputName) {
      throw new Error("Missing required params.");
    }
    this.compositeVertexId = compositeVertexId;
    this.inputName = inputName;
    this.ready = false;
    this.currentVertexId = null;
    this.currentVertexHandle = null;
    this.currentOffset = null;

    this.arcHandle = SK.arcs.watch({"to": {
      "vertexId": compositeVertexId,
      "ioName": inputName,
    }})
    .on("data", (arcs) => {
      if (arcs.length === 0) {
        this._clearArc();
      }
      else if (arcs.length === 1) {
        const [newArc] = arcs;
        this._handleArc(newArc);
      }
      else if (arcs.length > 1) {
        this.info("More than one arc connects to input!");
        this.info("SyncInputStream doing nothing until the confusion abates.");
      }
    });
  }

  _clearArc() {
    if (this.currentVertex) {
      this.info("Disconnected.");
      this.currentVertexId = null;
      this.currentVertexHandle.stop();
      this.currentOffset = null;
    }
  }

  info(text) {
    winston.info(`[SyncInputStream for ${this.inputName} of ${this.compositeVertexId}] ${text}`);
  }

  transformPTS(pts) {
    return pts + this.currentOffset;
  }

  transformDTS(dts) {
    return dts + this.currentOffset;
  }

  _handleArc(newArc) {
    const newVertexId = newArc.from.vertexId;
    if (newVertexId === this.currentVertexId) {
      // No change, chill. Just sit here.
      return;
    }
    this._clearArc();
    this.info(`Switching to vertex ${newVertexId}`);
    this.currentVertexId = newVertexId;
    this.currentVertexHandle = SK.vertices.watch({id: this.currentVertexId})
    .on("data", ([v]) => {
      if (!v.syncStartTime) {
        // Hasn't synced yet. No problem.
        return;
      }
      const offset = (v.syncStartTime - SERVER_START_TIME) * TIME_BASE;
      if (offset === this.currentOffset) {
        // Offset hasn't changed, that's cool.
        return;
      }
      this.currentOffset = offset;
      this.info(`Switching to offset ${this.currentOffset}`);
    })
    .catch(::this.info);
  }

  _transform(chunk, enc, next) {
    if (this.currentOffset === null) {
      next();
    }
    else {
      return super._transform(chunk, enc, next);
    }
  }
}
