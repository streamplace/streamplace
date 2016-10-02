
import React from "react";
import twixty from "twixtykit";

import SK from "../../SK";
import style from "./SceneRegionEditor.scss";

let uidIdx = 0;
const uid = function() {
  const label = `region-label-${uidIdx}`;
  uidIdx += 1;
  return label;
};

export default class SceneRegionEditor extends React.Component{
  constructor() {
    super();
    this.state = {};
  }

  componentDidMount() {
    this.subscribe(this.props);
  }

  subscribe(props) {
    this.setState({region: props.region});
    const inputId = props.region.inputId;
    if (this.inputHandle) {
      this.inputHandle.stop();
    }
    this.inputHandle = SK.inputs.watch({})
    .on("data", (inputs) => {
      this.setState({inputs});
    })
    .catch(::twixty.error);
  }

  componentWillUnmount() {
    if (this.inputHandle) {
      this.inputHandle.stop();
    }
  }

  handleChangeInput (field, e) {
    let val = e.target.value;
    const newRegion = {...this.state.region, [field]: val};
    this.setState({region: newRegion});
  }

  handleChange(field, e) {
    let val = parseInt(e.target.value);
    if (val !== val) { // NaN
      val = e.target.value;
    }
    const newRegion = {...this.state.region, [field]: val};
    this.setState({region: newRegion});
  }

  handleSave() {
    this.props.onChange(this.state.region);
  }

  handleDelete() {
    this.props.onDelete(this.state.region);
  }

  render () {
    if (!this.state.inputs) {
      return <div />;
    }
    const options = this.state.inputs.map((input) => {
      return <option value={input.id}>{input.title}</option>;
    });
    const fields = ["x", "y", "width", "height"].map((param) => {
      const handleChange = this.handleChange.bind(this, param);
      const id = uid();
      return (
        <span key={param}>
          <label className={style.InlineLabel} htmlFor={id}>{param}</label>
          <input id={id} className={style.MicroInput} type="text" value={this.state.region[param]} onChange={handleChange} />&nbsp;&nbsp;&nbsp;
        </span>
      );
    });
    return (
      <section className={style.RegionFieldContainer}>
        <select onChange={this.handleChangeInput.bind(this, "inputId")} className={style.InputSelector} value={this.state.region.inputId}>
          {options}
        </select>
        {fields}
        <button onClick={::this.handleSave}>Save</button>
        <button className="danger" onClick={::this.handleDelete}>Delete</button>
      </section>
    );
  }
}

SceneRegionEditor.propTypes = {
  "region": React.PropTypes.object.isRequired,
  "onChange": React.PropTypes.func.isRequired,
  "onDelete": React.PropTypes.func.isRequired,
};
