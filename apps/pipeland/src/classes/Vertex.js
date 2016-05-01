
import ffmpeg from "fluent-ffmpeg";
import stream from "stream";

import SK from "../sk";
import mpegts from "../mpegts-stream";
import Base from "./Base";

import RTMPInputVertex from "./vertices/RTMPInputVertex";
import RTMPOutputVertex from "./vertices/RTMPOutputVertex";
import MagicVertex from "./vertices/MagicVertex";
import ImageInputVertex from "./vertices/ImageInputVertex";

const Vertex = {};

const VertexMap = {
  "RTMPInput": RTMPInputVertex,
  "RTMPOutput": RTMPOutputVertex,
  "Magic": MagicVertex,
  "ImageInput": ImageInputVertex,
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
