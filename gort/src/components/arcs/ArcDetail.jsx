
import React from "react";
import twixty from "twixtykit";

import SK from "../../SK";
import style from "./ArcDetail.scss";

export default class ArcDetail extends React.Component {
  constructor(props) {
    super(props);
    this.displayName = "ArcDetail";
    this.state = {
      vertices: [],
      arc: {
        from: {},
        to: {}
      },
    };
  }
  componentDidMount() {
    this.doSubscriptions(this.props.broadcastId);
    if (this.props.create !== true) {
      if (!this.props.arcId) {
        throw new Error("ArcDetail called with create false but no arcId.");
      }
      this.arcHandle = SK.arcs.watch({id: this.props.arcId})
      .on("data", (arcs) => {
        this.setState({arc: arcs[0]});
      })
      .catch((err) => {
        twixty.error(err);
      });
    }
  }
  doSubscriptions(broadcastId) {
    this.vertexHandle = SK.vertices.watch({broadcastId})
    .on("data", (vertices) => {
      this.setState({vertices});
    })
    .catch((...args) => {
      twixty.error(...args);
    });

    // this.arcHandle = SK.arcs.watch({broadcastId})
    // .on("data", (arcs) => {
    //   this.setState({arcs});
    // })
    // .catch((...args) => {
    //   twixty.error(...args);
    // });
  }
  componentWillUnmount() {
    // this.arcHandle.stop();
    this.vertexHandle.stop();
    if (this.arcHandle) {
      this.arcHandle.stop();
    }
  }
  makeVertexList(field) {
    return this.state.vertices.map((v) => {
      const onClick = () => {
        const newArc = {...this.state.arc};
        newArc[field] = {
          vertexId: v.id,
          pipe: "default"
        };
        this.setState({arc: newArc});
        this.props.onChange(newArc);
      };
      let className = style.ArcListItem;
      if (this.state.arc[field].vertexId === v.id) {
        className = style.ArcListItemSelected;
      }
      return (
        <div key={v.id} className={className} onClick={onClick}>
          <span className={style.ArcListItemTitle}>{v.title}</span>
          <span className={style.ArcListItemDetails}>{v.type}</span>
        </div>
      );
    });
  }
  render() {
    return (
      <section className={style.ArcPicker}>
        <div className={style.ArcFrom}>
          {this.makeVertexList("from")}
        </div>
        <div className={style.ArcPointer}>
          <span className="fa fa-caret-square-o-right" />
        </div>
        <div className={style.ArcTo}>
          {this.makeVertexList("to")}
        </div>
      </section>
    );
  }
}

ArcDetail.propTypes = {
  broadcastId: React.PropTypes.string.isRequired,
  onChange: React.PropTypes.func.isRequired,
  create: React.PropTypes.bool,
  arcId: React.PropTypes.string,
};
