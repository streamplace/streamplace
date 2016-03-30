
import React from "react";

import BaseCreator from "./BaseCreator";
import style from "../VertexCreate.scss";

export default class DelayCreator extends BaseCreator {

  getDefaultVertex(params) {
    const v = super.getDefaultVertex(params);
    v.type = "Delay";
    v.inputs.default = {};
    v.outputs.video = {};
    v.outputs.audio = {};
    v.params.delayTime = 0;
    return v;
  }

  getFields(v) {
    return super.getFields(v).concat([
      <label key="params.rtmp.url" className={style.BlockLabel}>
        <span>Delay</span>
        <input type="text" value={v.params.delayTime} onChange={this.setField("params.delayTime")} />
      </label>
    ]);
  }
}
