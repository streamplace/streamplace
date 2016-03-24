
import React from "react";

import BaseCreator from "./BaseCreator";
import style from "../VertexCreate.scss";

export default class Combine2x1Creator extends BaseCreator {

  getDefaultVertex(params) {
    const v = super.getDefaultVertex(params);
    v.type = "Combine2x1";
    v.inputs.left = {};
    v.inputs.right = {};
    v.outputs.default = {};
    return v;
  }
}
