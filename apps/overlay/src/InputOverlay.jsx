
import React from "react";
import config from "sk-config";
import twixty from "twixtykit";
import IO from "socket.io-client";

import NotFound from "./NotFound";
import style from "./InputOverlay.scss";

// Quiet is global for now, so...
/*eslint-disable no-undef */

// TODO: implement our own timesync so we don't have to use theirs
const PUBLIC_TIMESYNC_SERVER_URL = config.require("PUBLIC_TIMESYNC_SERVER_URL");

const quietReady = new Promise((resolve, reject) => {
  Quiet.addReadyCallback(resolve, reject);
});

const PROFILE = {
  "checksum_scheme": "crc32",
  "inner_fec_scheme": "v27",
  "outer_fec_scheme": "none",
  "mod_scheme": "psk2",
  "frame_length": 25,
  "modulation": {
    "center_frequency": 4200,
    "gain": 0.15
  },
  "interpolation": {
    "samples_per_symbol": 10,
    "symbol_delay": 4,
    "excess_bandwidth": 0.35
  },
  "encoder_filters": {
    "dc_filter_alpha": 0.01
  },
  "resampler": {
    "delay": 13,
    "bandwidth": 0.45,
    "attenuation": 60,
    "filter_bank_size": 64
  }
};

export default class InputOverlay extends React.Component{
  constructor() {
    super();

    // Keep track of syncs we've already scheduled
    this.doneSyncFor = {};
    this.state = {
      input: {},
      notFound: false,
      infoText: ""
    };

    this.timeOffset = 0;
  }

  componentDidMount() {
    quietReady.then(() => {
      this.transmit = Quiet.transmitter(PROFILE);
      const overlayKey = this.props.params.overlayKey;
      const socketServer = `${PUBLIC_TIMESYNC_SERVER_URL}?overlayKey=${overlayKey}`;
      this.socket = IO(socketServer, {transports: ["websocket"]});
      this.socket.on("hello", () => {
        if (this.syncInterval) {
          clearInterval(this.syncInterval);
        }
        this.setState({infoText: ""});
        this.syncTime();
        this.syncInterval = setInterval(::this.syncTime, 2500);
      });
      this.socket.on("error", (err) => {
        // Socket.io gives us a string here, just give it to em
        this.setState({infoText: err});
      });
      this.socket.on("nextsync", (time) => {
        this.triggerSync(time);
      });
    });
  }

  syncTime() {
    const start = Date.now();
    this.socket.emit("timesync", "test", (serverTime) => {
      const end = Date.now();
      const halfTime = Math.floor((end - start) / 2) + start;
      this.timeOffset = serverTime - halfTime;
    });
  }

  now() {
    return Date.now() + this.timeOffset;
  }

  triggerSync(timestamp) {
    if (this.doneSyncFor[`${timestamp}`]) {
      // Already scheduled/performed this one
      return;
    }
    this.doneSyncFor[`${timestamp}`] = true;
    if (timestamp < this.now()) {
      // Nothing to be done, it already happened.
      return;
    }
    this.setState({infoText: `Syncing at ${timestamp}`});
    const buf = this._intToArray(timestamp);
    this._doAtExactTime(timestamp, () => {
      this.setState({infoText: "Sync time!"});
      this.transmit(buf, () => {
        this.setState({infoText: ""});
      });
    });
  }

  _intToArray(now) {
    const buf = new ArrayBuffer(8);
    const bufView = new Float64Array(buf);
    bufView[0] = now;
    return buf;
  }

  /**
   * Need to make noises at the exact right time, so we have this helper function to hone in.
   */
  _doAtExactTime(timestamp, cb) {
    const now = this.now();
    if (now >= timestamp) {
      cb();
    }
    else {
      // set a callback halfway to our goal
      let halfway = Math.floor((timestamp - now - 50)/2);
      if (halfway < 0) {
        halfway = 0;
      }
      this.setState({infoText: `Syncing in ${halfway}`});
      setTimeout(this._doAtExactTime.bind(this, timestamp, cb), halfway);
    }
  }

  componentWillUnmount() {
    this.inputHandle.stop();
  }

  render () {
    if (this.state.notFound) {
      return <NotFound />;
    }
    return (
      <section className={style.FullContainer}>
        <div className={style.TopRightBig}>{this.state.infoText}</div>
      </section>
    );
  }
}

InputOverlay.propTypes = {
  "params": React.PropTypes.object.isRequired,
};
