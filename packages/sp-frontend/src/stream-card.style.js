import styled from "styled-components";

const streamKinds = {
  Input: `
    background-color: #ffbfbf;
  `,
  File: `
    background-color: #c3c3ff;
  `
};

export const Card = styled.div`
  background-color: white;
  border: 1px solid #333;
  padding: 1em;

  ${props => streamKinds[props.kind]};
`;
