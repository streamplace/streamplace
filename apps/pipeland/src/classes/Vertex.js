
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
import MagicVertex from "./vertices/MagicVertex";
import ImpageInputVertex from "./vertices/ImageInputVertex";

const Vertex = {};

const VertexMap = {
  "RTMPInput": RTMPInputVertex,
  "RTMPOutput": RTMPOutputVertex,
  "Combine2x1": Combine2x1Vertex,
  "Combine2x2": Combine2x2Vertex,
  "Delay": DelayVertex,
  "AudioMix": AudioMixVertex,
  "Magic": MagicVertex,
  "ImageInput": ImpageInputVertex,
};

Vertex.create = function(params) {
  const {id, type} = params;
  const VertexClass = VertexMap[type];
  if (!VertexClass) {
    throw new Error(`Unknown Vertex Type: ${type}`);
  }
  return new VertexClass(params);
};

export default Vertex;
