
import React from "react";
import _ from "underscore";

import BaseCreator from "./BaseCreator";
import style from "../VertexCreate.scss";

export default class MagicCreator extends BaseCreator {

  getDefaultVertex(params) {
    const v = super.getDefaultVertex(params);
    v.type = "Magic";
    v.outputs.default = {};
    return v;
  }

  handleChange(field, e) {
    const newVertex = super.handleChange(field, e);
    if (field === "params.inputCount") {
      const count = parseInt(e.target.value);
      if (_(count).isNumber() && count >= 0) {
        newVertex.inputs = {};
        for (let i = 0; i < count; i++) {
          newVertex.inputs[`input${i}`] = {};
        }
      }
    }
  }

  getFields(v) {
    return super.getFields(v).concat([(
      <label key="params.inputCount" className={style.BlockLabel}>
        <span>Inputs</span>
        <input type="text" value={v.params.inputCount} onChange={this.setField("params.inputCount")} />
      </label>
    )]);
  }
}
