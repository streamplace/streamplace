
import React from "react";

import BroadcastList from "./broadcasts/BroadcastList";
import BroadcastCreate from "./broadcasts/BroadcastCreate";
import InputList from "./inputs/InputList";
import InputCreate from "./inputs/InputCreate";
import OutputList from "./outputs/OutputList";
import OutputCreate from "./outputs/OutputCreate";
import style from "./Home.scss";

export default class Home extends React.Component{
  constructor() {
    super();
    this.state = {};
  }

  render () {
    return (
      <div>
        <section className={style.MainColumns}>
          <div>
            <h3>Broadcasts</h3>
            <BroadcastList />
          </div>
          <div>
            <h3>Inputs</h3>
            <InputList />
          </div>
          <div>
            <h3>Outputs</h3>
            <OutputList />
          </div>
        </section>
      <section className={style.MainColumns}>
          <div>
            <BroadcastCreate />
          </div>
          <div>
            <InputCreate />
          </div>
          <div>
            <OutputCreate />
          </div>
        </section>
      </div>
    );
  }
}
