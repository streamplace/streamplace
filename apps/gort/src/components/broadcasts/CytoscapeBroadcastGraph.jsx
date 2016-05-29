
import React from "react";
import cytoscape from "cytoscape";
import _ from "underscore";
import twixty from "twixtykit";

import SK from "../../SK";
import style from "./CytoscapeBroadcastGraph.scss";

export default class CytoscapeBroadcastGraph extends React.Component{
  constructor() {
    super();
    this.state = {};
    this.handleResize = _.throttle(::this.handleResize, 500);
  }

  shouldComponentUpdate() {
    // Cytoscape will manage this, not React.
    return false;
  }

  componentDidMount() {
    window.addEventListener("resize", this.handleResize);
  }

  componentWillUnmount() {
    window.removeEventListener("resize", this.handleResize);
    if (this.vertexHandle) {
      this.vertexHandle.stop();
    }
    if (this.arcHandle) {
      this.arcHandle.stop();
    }
    if (this.cy) {
      this.cy.destroy();
    }
  }

  startCytoscape() {
    this.cy = cytoscape({
      container: document.getElementById("cytoscape"), // container to render in
      elements: [],
      userZoomingEnabled: false,
      userPanningEnabled: false,
      boxSelectionEnabled: false,
      autoungrabify: true,
      style: [{
        selector: "node",
        style: {
          "background-color": "white",
          "border-width": 5,
          "border-color": "grey",
          "label": "data(title)",
          "shape": "roundrectangle",
          "font-family": "Open Sans, Helvetica, sans-serif",
          "text-valign": "center",
          "text-halign": "center",
          "text-wrap": "wrap",
          "height": 75,
          width: Math.floor(75*16/9),
          "transition-property": "border-color",
          "transition-duration": "0.5s"
        }
      }, {
        selector: "edge",
        style: {
          "width": 10,
          "line-color": "#555",
          "target-arrow-color": "#555",
          "target-arrow-shape": "triangle"
        }
      }, {
        selector: ".selected",
        style: {
          "background-color": "lightblue"
        }
      }, {
        selector: ".selected-arc",
        style: {
          "line-color": "lightblue",
          "target-arrow-color": "lightblue"
        }
      }, {
        selector: ".active",
        style: {
          "border-color": "#2193E4"
        }

      }],
    });
    const broadcastId = this.props.broadcastId;
    this.vertexHandle = SK.vertices.watch({broadcastId})
    .on("newDoc", (vertex) => {
      this.cy.add({
        group: "nodes",
        data: {
          id: vertex.id,
          title: this.getVertexLabel(vertex)
        },
      });
      this.runLayout();
    })
    .on("deletedDoc", (vertex) => {
      const elem = this.cy.getElementById(vertex.id);
      this.cy.remove(elem);
    })
    .on("data", (vertices) => {
      vertices.forEach((vertex) => {
        const elem = this.cy.getElementById(vertex.id);
        if (elem.length === 0) {
          return;
        }
        elem.removeClass("active");
        if (vertex.status !== "WAITING") {
          elem.addClass("active");
        }
        elem.data("title", this.getVertexLabel(vertex));
      });
    })
    .catch((...args) => {
      twixty.error(...args);
    });

    this.arcHandle = SK.arcs.watch({broadcastId})
    .on("newDoc", (arc) => {
      this.cy.add({
        group: "edges",
        data: {
          id: arc.id,
          source: arc.from.vertexId,
          target: arc.to.vertexId
        },
      });
      this.runLayout();
    })
    .catch((...args) => {
      twixty.error(...args);
    });

    this.cy.on("tap", (e) => {
      this.cy.elements().removeClass("selected");
      this.cy.elements().removeClass("selected-arc");
      if (e.cyTarget === this.cy) { // background
        this.props.onPick(null);
      }
    });

    this.cy.on("tap", "node", (e) => {
      const target = e.cyTarget;
      target.addClass("selected");
      this.props.onPick("vertex", target.data("id"));
    });

    this.cy.on("tap", "edge", (e) => {
      const target = e.cyTarget;
      target.addClass("selected-arc");
      this.props.onPick("arc", target.data("id"));
    });

    this.cy.on("tap", "node", (e) => {
      const target = e.cyTarget;
      target.addClass("selected");
      this.props.onPick("vertex", target.data("id"));
    });
  }

  handleResize() {
    if (this.cy) {
      this.runLayout();
    }
  }

  getVertexLabel(vertex) {
    let line2 = vertex.timemark;
    if (!line2) {
      line2 = "Waiting...";
    }
    return [vertex.title, line2].join("\n");
  }

  runLayout() {
    this.cy.elements().layout({
      name: "breadthfirst",
      directed: true
    });
  }

  render () {
    setTimeout(::this.startCytoscape, 0);
    return (
      <div className={style.Container} id="cytoscape"></div>
    );
  }
}

CytoscapeBroadcastGraph.propTypes = {
  "broadcastId": React.PropTypes.string.isRequired,
  "onPick": React.PropTypes.func.isRequired,
};
