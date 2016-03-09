
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

  render() {
    return (
      <div className={style.ArcNode}>
        <div className={style.ArcNodeContent}>
          {this.state.from.title}<br />
          {this.state.to.title}
        </div>
      </div>
    );
  }
}

ArcGraphNode.propTypes = {
  arcId: React.PropTypes.string.isRequired,
};
