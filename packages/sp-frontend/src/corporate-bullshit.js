import React, { Component } from "react";
import styled, { injectGlobal } from "styled-components";
import logoUrl from "./streamplace-logo.svg";
import YouTube from "react-youtube";

const CorporateContainer = styled.section`
  display: flex;
  flex-direction: column;
  width: 100%;
  height: 100%;
`;

const Content = styled.div`
  max-width: 1000px;
  padding-left: 1em;
  padding-right: 1em;
  margin-left: auto;
  margin-right: auto;
  display: flex;
  justify-content: space-between;
`;

const Header = styled.header``;

const Title = styled.h1`
  font-weight: 200;
  font-size: 1.7em;
  margin-top: 0.4em;
  margin-bottom: 0.4em;
`;

const Logo = styled.img`
  height: 1.5em;
`;

const Hero = styled.section`
  background-color: black;
  display: flex;
  align-items: center;
  justify-content: center;
  position: relative;
  height: 300px;

  @media (min-width: 1000px) {
    height: 500px;
  }
`;

const Links = styled.nav`
  display: flex;
  align-items: stretch;
  justify-content: flex-end;
`;

const Link = styled.a`
  margin-left: 0.7em;
  color: #333;
  font-weight: 600;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  padding: 0 0.7em;
  &:hover {
    color: #666;
  }
`;

/* eslint-disable no-unused-expressions */
injectGlobal`
  #BigVideo {
    position: absolute;
    width: 100%;
    height: 100%;
    top: 0;
    left: 0;
  }
`;

const Icon = styled.i`
  &.fa {
    font-size: 2em;
  }
`;

export default class CorporateBullshit extends Component {
  renderYoutube() {
    const youtubeOpts = {
      playerVars: {
        autoplay: 1,
        modestbranding: 1,
        channel: "UC_0VBEHybbwCoaJHsUmQZEg"
      }
    };
    return <YouTube id="BigVideo" videoId="live_stream" opts={youtubeOpts} />;
  }

  render() {
    return (
      <CorporateContainer>
        <Header>
          <Content>
            <Title>
              <Logo src={logoUrl} alt="Streamplace" title="Streamplace" />
            </Title>
            <Links>
              <Link href="https://github.com/streamplace/streamplace">
                <Icon className="fa fa-github" aria-hidden="true" />
              </Link>
              <Link href="https://twitter.com/streamplace">
                <Icon className="fa fa-twitter" aria-hidden="true" />
              </Link>
              <Link href={this.props.loginUrl}>LOG IN</Link>
            </Links>
          </Content>
          <Hero>{this.renderYoutube()}</Hero>
        </Header>
      </CorporateContainer>
    );
  }
}
