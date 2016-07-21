
import React from "react";

import style from "./NotFound.scss";

export default class NotFound extends React.Component{
  constructor() {
    super();
    this.state = {};
  }

  render () {
    return (
      <section className={style.FullContainer}>
        <div className={style.BigText}>
          Input not found. Check overlay URL.
        </div>
      </section>
    );
  }
}

NotFound.propTypes = {
};
