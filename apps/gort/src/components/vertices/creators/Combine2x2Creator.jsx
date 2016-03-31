
import React from "react";

import BaseCreator from "./BaseCreator";
import style from "../VertexCreate.scss";

export default class Combine2x2Creator extends BaseCreator {

  getDefaultVertex(params) {
    const v = super.getDefaultVertex(params);
    v.type = "Combine2x2";
    v.inputs.topleft = {};
    v.inputs.topright = {};
    v.inputs.bottomleft = {};
    v.inputs.bottomright = {};
    v.outputs.default = {};
    return v;
  }
}
