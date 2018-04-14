import React, { Component } from "react";
import styled, { injectGlobal } from "styled-components";
import logoUrl from "./streamplace-logo.svg";
import YouTube from "react-youtube";
import droneLogo from "./drone.svg";
import logo from "./icon.svg";

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
  color: black;
  font-weight: 600;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  padding: 0 0.7em;
  opacity: 0.8;
  &:hover {
    opacity: 0.6;
  }
`;

const Icon = styled.i`
  &.fa {
    font-size: 2em;
  }
`;

const DroneLogo = styled.img`
  height: 1.8em;
`;

const Downloads = styled.div`
  display: flex;
  padding: 1em;
  flex-grow: 1;
  justify-content: space-around;
`;

const Platform = styled.a`
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  text-align: center;
  color: black;
  margin: 1em;
  padding: 1em;
`;

const downloadWidth = "125px;";

const SquareBG = styled.div`
  background-color: #333333;
  width: ${downloadWidth};
  margin-left: 0;
  margin-right: 0;
`;

const SizedLogo = styled.img`
  width: ${downloadWidth};
`;

const DownloadButton = styled.span`
  border-radius: 5px;
  border: 1px solid #cccccc;
  margin-top: 1em;
  color: black;
  padding: 1em;
  font-weight: bold;
  width: ${downloadWidth};
`;

/* eslint-disable no-unused-expressions */
// we want to only do global hacks if we're actually loaded ever.
let injected = false;
const injectGlobalBullshitHacks = () => {
  if (injected) {
    return;
  }
  injectGlobal`
    a {
      text-decoration: none;
    }
    #BigVideo {
      position: absolute;
      width: 100%;
      height: 100%;
      top: 0;
      left: 0;
    }
  `;
  injected = true;
};

export default class CorporateBullshit extends Component {
  constructor() {
    super();
    injectGlobalBullshitHacks();
  }
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
              <Link href="https://www.youtube.com/channel/UC_0VBEHybbwCoaJHsUmQZEg">
                <Icon className="fa fa-youtube-play" aria-hidden="true" />
              </Link>
              <Link href="https://github.com/streamplace/streamplace">
                <Icon className="fa fa-github" aria-hidden="true" />
              </Link>
              <Link href="https://twitter.com/streamplace">
                <Icon className="fa fa-twitter" aria-hidden="true" />
              </Link>
              <Link href="https://drone.stream.place">
                <DroneLogo src={droneLogo} />
              </Link>
              <Link
                style={{ cursor: "pointer" }}
                onClick={() => this.props.onLogin()}
              >
                LOG IN
              </Link>
            </Links>
          </Content>
          <Hero>{this.renderYoutube()}</Hero>
          <Downloads>
            <Platform href="https://stream.place/dl/Streamplace%20Setup.exe">
              <h4>Streamplace for Windows</h4>
              <SquareBG>
                <SizedLogo src={logo} />
              </SquareBG>
              <DownloadButton>Download</DownloadButton>
            </Platform>
            <Platform href="https://stream.place/dl/Streamplace.dmg">
              <h4>Streamplace for macOS</h4>
              <SizedLogo src={logo} />
              <DownloadButton>Download</DownloadButton>
            </Platform>
          </Downloads>
        </Header>
      </CorporateContainer>
    );
  }
}
