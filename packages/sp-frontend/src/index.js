import React from "react";
import ReactDOM from "react-dom";
import "./index.css";

(async function() {
  const isElectron =
    window && window.process && window.process.type === "renderer";
  if (isElectron) {
    const SPAppFrontend = await import("./sp-app-frontend");
    SPAppFrontend.default();
  } else {
    const SPFrontend = (await import("./sp-frontend")).default;
    ReactDOM.render(<SPFrontend />, document.querySelector("main"));
  }
})();
