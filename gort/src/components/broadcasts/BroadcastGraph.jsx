
import React from "react";
import twixty from "twixtykit";
import _ from "underscore";

import SK from "../../SK";
import ArcGraphNode from "../arcs/ArcGraphNode";
import VertexGraphNode from "../vertices/VertexGraphNode";
import style from "./BroadcastGraph.scss";

export default class BroadcastGraph extends React.Component {
  constructor(props) {
    super(props);
    this.state = {
      vertices: [],
      arcs: [],
      selected: {
        type: null,
        id: null,
      },
    };
  }
  /**
   * Traverse the graph and return an array with it neatly divided into columns.
   * @return {Array}
   */
  traverseGraph(vertices, arcs) {
    const groupings = [];
    // We don't wanna modify their arrays, so copy everything quick

    const vertexIdx = _(vertices).indexBy("id");
    const vertexToArc = {};
    vertices.forEach(function(v) {
      vertexToArc[v.id] = [];
    });
    arcs.forEach(function(arc) {
      [arc.from.vertexId, arc.to.vertexId].forEach((vertexId) => {
        vertexToArc[vertexId].push(arc);
      });
    });

    const outstanding = new Set([...vertices, ...arcs]);

    while (outstanding.size > 0) {
      const startVertex = outstanding.values().next().value;
      // Stores tuples of [idx, thingy]
      let results = [];
      const queue = [[0, startVertex]];
      let minimum = 0;
      while (queue.length > 0) {
        const [col, item] = queue.pop();
        // If someone else already proccessed us, cool. GO ON.
        if (!outstanding.has(item)) {
          continue;
        }
        outstanding.delete(item);
        if (col < minimum) {
          minimum = col;
        }
        results.push([col, item]);
        if (item.kind === "vertex") {

          // Add my arcs to the queue
          vertexToArc[item.id].forEach(function(arc) {
            if (arc.from.vertexId === item.id) {
              queue.push([col + 1, arc]);
            }
            else if (arc.to.vertexId === item.id) {
              queue.push([col - 1, arc]);
            }
          });
        }
        else if (item.kind === "arc") {
          const fromVertex = vertexIdx[item.from.vertexId];
          queue.push([col - 1, fromVertex]);
          const toVertex = vertexIdx[item.to.vertexId];
          queue.push([col + 1, toVertex]);
        }
        else {
          throw new Error("Unknown type in traverseGraph: " + item.kind);
        }
      }
      // Get the absolute value so we can normalize the rest of these suckers
      minimum = Math.abs(minimum);
      const columns = [];
      results.forEach(([col, item]) => {
        const resultColumn = col + minimum;
        if (!columns[resultColumn]) {
          columns[resultColumn] = [];
        }
        columns[resultColumn].push(item);
      });
      groupings.push(columns);
    }
    return groupings;
  }
  componentDidMount() {
    const broadcastId = this.props.broadcastId;
    this.vertexHandle = SK.vertices.watch({broadcastId})
    .on("data", (vertices) => {
      if (vertices.length !== this.state.vertices.length) {
        this.setState({vertices});
      }
    })
    .catch((...args) => {
      twixty.error(...args);
    });

    this.arcHandle = SK.arcs.watch({broadcastId})
    .on("data", (arcs) => {
      if (arcs.length !== this.state.arcs.length) {
        this.setState({arcs});
      }
    })
    .catch((...args) => {
      twixty.error(...args);
    });
  }
  componentWillUnmount() {
    this.arcHandle.stop();
    this.vertexHandle.stop();
  }
  handleClick(type, id) {
    this.setState({selected: {type, id}});
    this.props.onPick(type, id);
  }
  handleBackgroundClick(e) {
    // If they clicked the background itself, deselect everything
    if (e.target === e.currentTarget) {
      this.setState({
        selected: {type: null, id: null}
      });
      this.props.onPick(null, null);
    }
  }
  render() {
    const graph = this.traverseGraph(this.state.vertices, this.state.arcs);
    const renderedGroupings = graph.map((group, idx) => {
      const renderedColumns = group.map((column, idx) => {
        const renderedItems = column.map((item) => {
          const selected = this.state.selected.id === item.id;
          const onClick = this.handleClick.bind(this, item.kind, item.id);
          let E;
          if (item.kind === "arc") {
            return <ArcGraphNode onClick={onClick} selected={selected} arcId={item.id} key={item.id} />;
          }
          else if (item.kind === "vertex") {
            return <VertexGraphNode onClick={onClick} selected={selected} vertexId={item.id} key={item.id} />;
          }
        });
        return <div className={style.BroadcastGraphColumn} key={idx}>{renderedItems}</div>;
      });
      return <div className={style.BroadcastGraphGrouping} key={idx}>{renderedColumns}</div>;
    });
    return (
      <div onClick={this.handleBackgroundClick.bind(this)} className={style.BroadcastGraph}>
        {renderedGroupings}
      </div>
    );
  }
}

BroadcastGraph.propTypes = {
  broadcastId: React.PropTypes.string.isRequired,
  onPick: React.PropTypes.func.isRequired
};
