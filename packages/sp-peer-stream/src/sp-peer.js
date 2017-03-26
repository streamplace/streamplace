
import SP from "sp-client";
import EE from "wolfy87-eventemitter";
import {getPeer} from "./sp-peer-stream";
import SPPeerConnection from "./sp-peer-connection";

export default class SPPeer extends EE {
  constructor(userId) {
    super();
    this.userId = userId;
    if (!SP.user) {
      throw new Error("Cannot create peer connection without being logged in!");
    }
    this.on("stream", (stream) => {
      this.stream = stream;
    });
    if (userId === SP.user.id) {
      this.startLocalStream();
    }
    else {
      this.startRemoteStream();
    }
  }

  /**
   * This is a peer connection to the current user, cool. Set up a local stream.
   */
  startLocalStream() {
    return navigator.mediaDevices.getUserMedia({
      audio: true,
      video: {
        width: {min: 1280},
        height: {min: 720},
      },
    })
    .then((stream) => {
      this.emit("stream", stream);
    });
  }

  /**
   * Open up a PeerConnection to the other person, plz
   */
  startRemoteStream() {
    const localPeer = getPeer(SP.user.id);
    localPeer.getStream().then((stream) => {
      this.connection = new SPPeerConnection({
        targetUserId: this.userId,
        stream: stream,
      });
      this.connection.getStream().then((stream) => {
        this.emit("stream", stream);
      });
    });
  }

  /**
   * Get an active stream if we have one. Otherwise resolve one when we do.
   */
  getStream() {
    if (this.stream) {
      return Promise.resolve(this.stream);
    }
    else {
      return new Promise((resolve, reject) => {
        this.once("stream", (stream) => {
          resolve(stream);
        });
      });
    }
  }
}
