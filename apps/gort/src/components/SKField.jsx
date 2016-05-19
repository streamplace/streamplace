
import React from "react";
import dot from "dot-object";

let currentId = 0;

export default class SKField extends React.Component{
  constructor(props) {
    super();
    this.state = {};
    this.id = `sk-field-${currentId}`;
    currentId += 1;
  }

  handleChange(e) {
    const newData = {...this.props.data};
    newData[this.props.field] = e.target.value;
    this.props.onChange(newData);
  }

  render () {
    const value = dot.pick(this.props.field, this.props.data);
    return (
      <div>
        <label htmlFor={this.id}>{this.props.label}</label>
        <input id={this.id} onChange={this.handleChange.bind(this)} type="text" placeholder={this.props.placeholder} value={value} />
      </div>
    );
  }
}

SKField.propTypes = {
  "onChange": React.PropTypes.func.isRequired,
  "data": React.PropTypes.object.isRequired,
  "field": React.PropTypes.string.isRequired,
  "type": React.PropTypes.string.isRequired,
  "label": React.PropTypes.string.isRequired,
  "placeholder": React.PropTypes.string,
};
