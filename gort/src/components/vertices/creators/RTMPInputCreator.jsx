
import React from "react";

import BaseCreator from "./BaseCreator";
import style from "../VertexCreate.scss";

export default class RTMPInputCreator extends BaseCreator {

  getDefaultVertex(params) {
    const v = super.getDefaultVertex(params);
    v.type = "RTMPInput";
    v.outputs.default = {};
    v.params.rtmp = {
      url: ""
    };
    return v;
  }

  getFields(v) {
    return super.getFields(v).concat([
      <label key="params.rtmp.url" className={style.BlockLabel}>
        <span>RTMP URL</span>
        <input type="text" value={v.params.rtmp.url} onChange={this.setField("params.rtmp.url")} />
      </label>
    ]);
  }
}
