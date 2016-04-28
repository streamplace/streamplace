
import React from "react";

import BaseCreator from "./BaseCreator";
import style from "../VertexCreate.scss";

export default class ImageInputCreator extends BaseCreator {

  getDefaultVertex(params) {
    const v = super.getDefaultVertex(params);
    v.type = "ImageInput";
    v.outputs = [{
      name: "default",
      sockets: [{
        type: "video"
      }]
    }];
    v.params.url = "";
    return v;
  }

  getFields(v) {
    return super.getFields(v).concat([(
      <label key="params.url" className={style.BlockLabel}>
        <span>URL</span>
        <input type="text" value={v.params.url} onChange={this.setField("params.url")} />
      </label>
    )]);
  }
}
