import styled, { injectGlobal } from "styled-components";

export const activeColor = "#00b8ff";

/* eslint-disable no-unused-expressions */
injectGlobal`
  a {
    text-decoration: none;
    cursor: pointer;
    color: ${activeColor};
  }
`;

export const FlexContainer = styled.div`
  width: 100%;
  height: 100%;
  flex-grow: 1;
  display: flex;
  position: relative;

  flex-direction: ${props => (props.column ? "column" : "row")};
  justify-content: ${props => props.justifyContent || "flex-start"}
    ${props => (props.padded ? "padding: 1em;" : "")};
`;

export const NiceForm = styled.form`
  padding: 1em;
  background-color: #eee;
`;

export const NiceLabel = styled.label`
  color: #222;
  margin-top: 0.5em;
  display: block;
`;

export const NiceTitle = styled.h4`margin-top: 0;`;

export const BigInput = styled.input`
  border: none;
  border-radius: 5px;
  border: 1px solid #ccc;
  background-color: white;
  font-size: 1.1em;
  padding: 0.4em;
  display: block;
  margin: 0.3em 0;
  font-weight: 200;
  width: 100%;

  &:focus,
  &:active {
    border-color: ${activeColor};
    outline: none;
  }
`;

export const NiceSubmit = styled.button`
  border: none;
  background-color: transparent;
  font-size: 1.1em;
  margin-top: 0.5em;
  color: ${activeColor};
  cursor: pointer;
`;
