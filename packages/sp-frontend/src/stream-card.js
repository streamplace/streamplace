import React, { Component } from "react";
import { watch, bindComponent } from "sp-components";
import { Card } from "./stream-card.style";
import { FlexContainer } from "./shared.style";
import Loading from "./loading";
import Timecode from "./timecode";

const TIMEBASE = 90000 / 60;
const ASSUME_STREAM_DEAD = 20000; // After this, it's probably gone forever

export class StreamCard extends Component {
  static propTypes = {
    kind: React.PropTypes.string.isRequired,
    id: React.PropTypes.string.isRequired,
    source: React.PropTypes.object,
    streams: React.PropTypes.array
  };

  static subscribe(props) {
    const tableName = props.SP.schema.definitions[props.kind].tableName;
    return {
      source: watch.one(tableName, { id: props.id }),
      streams: watch("streams", { source: { id: props.id } })
    };
  }

  // These need to update periodically to detect dead streams and such.
  componentDidMount() {
    this.interval = setInterval(() => {
      this.setState({ now: Date.now() });
    }, 500);
  }

  componentWillUnmount() {
    clearInterval(this.interval);
  }

  render() {
    if (!this.props.source || !this.props.streams) {
      return (
        <Card>
          <Loading />
        </Card>
      );
    }
    const streams = this.props.streams.filter(stream => {
      return Date.now() - stream.timestamp.time < ASSUME_STREAM_DEAD;
    });
    return (
      <Card {...this.props}>
        <FlexContainer justifyContent="space-between">
          {this.props.source.title || this.props.source.name}
          {streams.map(stream =>
            <div key={stream.id}>
              <Timecode
                pts={stream.timestamp.pts}
                time={stream.timestamp.time}
              />
            </div>
          )}
          {streams.length === 0 && <div>Offline</div>}
        </FlexContainer>
      </Card>
    );
  }
}

export default bindComponent(StreamCard);
