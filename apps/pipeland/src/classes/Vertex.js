
import ffmpeg from "fluent-ffmpeg";
import stream from "stream";

import SK from "../sk";
import mpegts from "../mpegts-stream";
import Base from "./Base";

import RTMPInputVertex from "./vertices/RTMPInputVertex";
import RTMPOutputVertex from "./vertices/RTMPOutputVertex";
import Combine2x1Vertex from "./vertices/Combine2x1Vertex";
import DelayVertex from "./vertices/DelayVertex";

const Vertex = {};

Vertex.create = function(params) {
  const {id, type} = params;
  if (type === "RTMPInput") {
    return new RTMPInputVertex(params);
  }
  else if (type === "RTMPOutput") {
    return new RTMPOutputVertex(params);
  }
  else if (type === "Combine2x1") {
    return new Combine2x1Vertex(params);
  }
  else if (type === "Delay") {
    return new DelayVertex(params);
  }
  else {
    throw new Error(`Unknown Vertex Type: ${type}`);
  }
};

export default Vertex;
