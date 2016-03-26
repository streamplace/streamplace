
import React from "react";
import twixty from "twixtykit";

import SK from "../../SK";
import style from "./ArcGraphNode.scss";

export default class ArcGraphNode extends React.Component {
  constructor(props) {
    super(props);
    this.state = {
      arc: {},
      from: {},
      to: {},
    };
  }

  componentDidMount() {
    const arcId = this.props.arcId;
    this.arcHandle = SK.arcs.watch({id: arcId})
    .on("data", (arcs) => {
      const arc = arcs[0];
      this.setState({arc});
      // Get details of our vertices
      ["from", "to"].forEach((field) => {
        SK.vertices.findOne(arc[field].vertexId).then((vertex) => {
          this.setState({[field]: vertex});
        });
      });
    })
    .catch(function(err) {
      twixty.error(err);
    });
  }

  componentWillUnmount() {
    this.arcHandle.stop();
  }

  handleClick() {
    this.props.onClick && this.props.onClick();
  }

  render() {
    let className = style.ArcNode;
    if (this.props.selected) {
      className = style.ArcNodeSelected;
    }
    return (
      <div className={className} onClick={this.handleClick.bind(this)}>
        <div className={style.ArcNodeContent}>
          {this.state.from.title}<br />
          {this.state.to.title}<br />
          {this.state.arc.size}
        </div>
      </div>
    );
  }
}

ArcGraphNode.propTypes = {
  arcId: React.PropTypes.string.isRequired,
  selected: React.PropTypes.bool,
  onClick: React.PropTypes.func,
};
