
import React from "react";
import _ from "underscore";

import BaseCreator from "./BaseCreator";
import VertexDetail from "../VertexDetail";
import style from "../VertexCreate.scss";

export default class MagicCreator extends BaseCreator {

  getDefaultVertex(params) {
    const v = super.getDefaultVertex(params);
    v.type = "Magic";
    v.outputs = [{
      name: "default",
      sockets: [{
        type: "video"
      }, {
        type: "audio"
      }]
    }],
    v.params.positions = {};
    return v;
  }

  handleChange(field, e) {
    const newVertex = super.handleChange(field, e);
    if (field === "params.inputCount") {
      const count = parseInt(e.target.value);
      if (_(count).isNumber() && count >= 0) {
        newVertex.inputs = [{
          name: "background",
          sockets: [{
            type: "video"
          }]
        }];
        for (let i = 0; i < count; i++) {
          newVertex.inputs.push({
            name: `input${i}`,
            sockets: [{
              type: "video"
            }, {
              type: "audio"
            }]
          });
          newVertex.params.positions[`input${i}`] = {
            x: 0,
            y: 0,
            width: 640,
            height: 480,
          };
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
