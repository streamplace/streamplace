
import ffmpeg from "fluent-ffmpeg";
import stream from "stream";

import SK from "../sk";
import mpegts from "../mpegts-stream";
import Base from "./Base";

import RTMPInputVertex from "./vertices/RTMPInputVertex";
import RTMPOutputVertex from "./vertices/RTMPOutputVertex";
import Combine2x1Vertex from "./vertices/Combine2x1Vertex";
import Combine2x2Vertex from "./vertices/Combine2x2Vertex";
import DelayVertex from "./vertices/DelayVertex";
import AudioMixVertex from "./vertices/AudioMixVertex";

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
  else if (type === "Combine2x2") {
    return new Combine2x2Vertex(params);
  }
  else if (type === "Delay") {
    return new DelayVertex(params);
  }
  else if (type === "AudioMix") {
    return new AudioMixVertex(params);
  }
  else {
    throw new Error(`Unknown Vertex Type: ${type}`);
  }
};

export default Vertex;
