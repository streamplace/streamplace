import React from "react";
import ReactDOM from "react-dom";
import "./index.css";
import CorporateBullshit from "./corporate-bullshit";
import { injectGlobal } from "styled-components";

injectGlobal`
  body {
    font-family: "Open Sans", Helvetica, Arial, sans-serif;
    margin: 0;
  }
`;

ReactDOM.render(<CorporateBullshit />, document.querySelector("main"));

// Uncomment me when it's go time:

// import StreamplaceUI from "streamplace-ui";

// ReactDOM.render(<StreamplaceUI />, document.querySelector("main"));
