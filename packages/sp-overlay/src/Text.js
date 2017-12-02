import styled from "styled-components";

const DEF_SIZE = 200;

export const shadow = (size, color = "white") => {
  if (!size) {
    size = DEF_SIZE;
  }
  const sw = Math.floor(size / 40);
  return `
    ${sw}px ${sw}px 1px white,
    -${sw}px ${sw}px 1px white,
    ${sw}px -${sw}px 1px white,
    -${sw}px -${sw}px 1px white
  `;
};

const Text = styled.div`
  margin: 0;
  display: inline;
  position: absolute;
  width: 999999px;
  height: 99999px;
  top: ${props => props.top}px;
  left: ${props => props.left}px;
  font-size: ${props => props.fontSize || DEF_SIZE}px;
  color: black;
  text-shadow: ${props => shadow(props.fontSize)};
  opacity: 1;
`;

export default Text;
