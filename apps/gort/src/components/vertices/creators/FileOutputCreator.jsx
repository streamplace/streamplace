
import React from "react";

import BaseCreator from "./BaseCreator";
import style from "../VertexCreate.scss";

export default class FileOutputCreator extends BaseCreator {

  getDefaultVertex(params) {
    const v = super.getDefaultVertex(params);
    v.type = "FileOutput";
    v.inputs = [{
      name: "default",
      sockets: [{
        type: "video"
      }, {
        type: "audio"
      }]
    }];
    return v;
  }
}
