
import React from "react";
import { Mote } from "bellamie";
import style from "./MoteDetail.scss";

export default React.createClass({
  render() {
    return (
      <section>
        <span className={style.noMoteSelected}> - No Mote Selected - </span>
      </section>
    );
  }
});
