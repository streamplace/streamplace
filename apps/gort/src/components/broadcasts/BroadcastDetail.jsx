
import React from "react";
import { Link } from "react-router";
import twixty from "twixtykit";

import BroadcastGraph from "./BroadcastGraph";
import SceneEditor from "../scenes/SceneEditor";
import VertexCreate from "../vertices/VertexCreate";
import VertexDetail from "../vertices/VertexDetail";
import ArcEdit from "../arcs/ArcEdit";
import Loading from "../Loading";
import style from "./BroadcastDetail.scss";
import SK from "../../SK";

export default class BroadcastDetail extends React.Component {
  constructor(props) {
    super(props);
    this.state = {
      broadcast: {},
      showNewVertex: false,
      selected: null
    };
  }

  componentDidMount() {
    const broadcastId = this.props.params.broadcastId;
    this.broadcastHandle = SK.broadcasts.watch({id: broadcastId})
    .on("data", (broadcasts) => {
      this.setState({broadcast: broadcasts[0]});
    })
    .catch((...args) => {
      twixty.error(...args);
    });
  }

  componentWillUnmount() {
    this.broadcastHandle.stop();
  }

  handleCloseBottomPanelClick() {

  }

  toggleEnabled() {
    const enabled = !this.state.broadcast.enabled;
    SK.broadcasts.update(this.state.broadcast.id, {enabled})
    .catch((err) => {
      twixty.error(err);
    });
  }

  // Not sure if this is kosher. Works great though.
  getSelected(tab) {
    for (let i = 0; i < this.props.routes.length; i+=1) {
      if (this.props.routes[i].path.indexOf(tab) === 0) {
        return style.TabContainerSelected;
      }
    }
    return "";
  }

  render() {
    if (!this.state.broadcast.id) {
      return <Loading />;
    }

    let toggleEnabledButton;
    const toggle = this.toggleEnabled.bind(this);
    if (this.state.broadcast.enabled === true) {
      toggleEnabledButton = (
        <button onClick={toggle} className={style.ToggleEnabledButton}>
          Stop Broadcast
        </button>
      );
    }
    else {
      toggleEnabledButton = (
        <button onClick={toggle} className={style.ToggleEnabledButton}>
          Start Broadcast
        </button>
      );
    }

    return (
      <section className={style.verticalPanels}>
        <section className={style.header}>
          <nav className={style.TabContainer}>
            <Link className={this.getSelected("scenes")} to={`/broadcasts/${this.props.params.broadcastId}/scenes`}>
              <span>Scene Editor</span>
            </Link>
            <Link className={this.getSelected("outputs")} to={`/broadcasts/${this.props.params.broadcastId}/outputs`}>
              <span>Outputs</span>
            </Link>
            <Link className={this.getSelected("graph")} to={`/broadcasts/${this.props.params.broadcastId}/graph`}>
              <span>Graph View</span>
            </Link>
          </nav>
          <em className={style.BroadcastName}>{this.state.broadcast.title}</em>
          {toggleEnabledButton}
        </section>

        {this.props.children}
      </section>
    );
  }
}

BroadcastDetail.propTypes = {
  "params": React.PropTypes.object.isRequired,
  "routes": React.PropTypes.object.isRequired,
  "children": React.PropTypes.object.isRequired,
};
