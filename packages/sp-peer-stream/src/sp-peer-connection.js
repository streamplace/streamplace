/**
 * Wrapper for Kurento's PeerConnection, which in turn wraps and abstracts the actual WebRTC
 * interfaces.
 */

import debug from "debug";
import SP from "sp-client";
import EE from "wolfy87-eventemitter";

const log = debug("sp:sp-peer-connection");
const kurentoLog = debug("sp:sp-peer-kurento");

// This is the only way we get any control of Kurento's logger... see also index.html
if (typeof window.Logger === "object") {
  Object.keys(console).forEach(key => {
    window.Logger[key] = kurentoLog;
  });
}

import KurentoUtils from "kurento-utils";
import { parse as parseUrl } from "url";

if (typeof RTCPeerConnection === "undefined") {
  if (typeof webkitRTCPeerConnection !== "undefined") {
    window.RTCPeerConnection = window.webkitRTCPeerConnection;
  } else if (typeof mozRTCPeerConnection !== "undefined") {
    window.RTCPeerConnection = window.mozRTCPeerConnection;
  } else {
    SP.error("No RTCPeerConnection implementation found");
  }
}

const PEER_CONNECTION_EVENTS = [
  "addstream",
  "icecandidate",
  "iceconnectionstatechange",
  "icegatheringstatechange",
  "negotiationneeded",
  "removestream",
  "signalingstatechange",
  "datachannel",
  "close",
  "error",
  "message",
  "open",
  "tonechange",
  "identityresult",
  "idpassertionerror",
  "idpvalidationerror",
  "peeridentity",
  "isolationchange"
];

export default class SPPeerConnection extends EE {
  constructor({ targetUserId, stream }) {
    super();
    this.isShutDown = false;
    this.targetUserId = targetUserId;
    this.videoStream = stream;
    this.localIceCandidates = [];
    this.remoteIceCandidatesChecked = [];
    this.processedOffer = false;
    this.kurentoIsSetup = false;
    this.options = {
      mediaConstraints: {
        audio: true,
        video: {
          width: 1280,
          height: 720
        }
      },
      onicecandidate: ::this.onIceCandidate,
      configuration: {
        iceTransportPolicy: "relay"
      }
    };

    this.streamPromise = new Promise((resolve, reject) => {
      this._streamResolve = resolve;
      this._streamReject = reject;
    });

    this.isPrimaryUser = SP.user.id < targetUserId;
    if (this.isPrimaryUser) {
      this.createOffer();
    } else {
      this.waitForOffer();
    }

    // Set a timeout that retries everything if nothing's worked. And do it randomly, so you don't
    // get two peers timing out and retrying at the same time.
    this.timeoutMs = 5000 + Math.floor(Math.random() * 5000);
    this.resetTimeout();
    this.shouldTimeout = true;
  }

  /**
   * Reset the auto-retry timeout
   */
  resetTimeout() {
    if (!this.shouldTimeout) {
      return;
    }
    if (this.timeoutHandle) {
      clearTimeout(this.timeoutHandle);
    }
    this.timeoutHandle = setTimeout(() => {
      log(`Hit timeout of ${this.timeoutMs}ms, aborting connection.`);
      this.shutdown();
    }, this.timeoutMs);
  }

  /**
   * Stop the auto-retry timeout
   */
  stopTimeout() {
    if (this.timeoutHandle) {
      clearTimeout(this.timeoutHandle);
    }
    this.shouldTimeout = false;
  }

  /**
   * Stop everything about this connection and clean everything up.
   */
  shutdown() {
    if (this.isShutDown === true) {
      return;
    }
    this.stopTimeout();
    this.webRtcPeer && this.webRtcPeer.dispose();
    this.isShutDown = true;
    this.peerHandle && this.peerHandle.stop();
    this.emit("disconnected");
    if (this.peer) {
      SP.peerconnections.delete(this.peer.id);
    }
  }

  _generateKurentoPeer() {
    return new Promise((resolve, reject) => {
      log("Creating Kurento peer");
      this.webRtcPeer = KurentoUtils.WebRtcPeer.WebRtcPeerSendrecv(
        this.options,
        error => {
          this.webRtcPeer.peerConnection.addEventListener(
            "iceconnectionstatechange",
            ::this.handleConnectionChange
          );
          if (error) {
            return reject(error);
          }
          this.kurentoIsSetup = true;
          resolve();
        }
      );
    });
  }

  handleConnectionChange(e) {
    const state = e.currentTarget.iceConnectionState;
    log(`ICE Connection State: ${state}`);
    if (state === "connected") {
      this.stopTimeout();
    }
    if (state === "completed") {
      // Cool, we're connected, no reason to keep that peerconnection around anymore
      this.peerHandle.stop();
      // Commented for now; needs work. Things get deleted at the wrong time and cycle.
      // if (this.isPrimaryUser) {
      //   SP.peerconnections.delete(this.peer.id).catch(log);
      // }
    }
    if (state === "failed" || state === "disconnected" || state === "closed") {
      this.shutdown();
    }
  }

  /**
   * We're the smaller one. Create an API object for it.
   */
  createOffer() {
    log("Creating peerconnection");
    return SP.peerconnections
      .create({
        userId: SP.user.id,
        targetUserId: this.targetUserId
      })
      .then(peer => {
        this.resetTimeout();
        this.peer = peer;
        this.peerHandle = SP.peerconnections.watch({ id: peer.id });
        this.peerHandle.on("data", ::this.peerUpdate);
        return this.setupTurnUrls();
      })
      .then(() => {
        this.resetTimeout();
        log("Generating offer");
        return new Promise((resolve, reject) => {
          this.localIceCandidates = [];
          this.webRtcPeer.generateOffer((err, offer) => {
            if (err) {
              return reject(err);
            }
            resolve(offer);
          });
        });
      })
      .then(offer => {
        this.resetTimeout();
        return SP.peerconnections.update(this.peer.id, {
          sdpOffer: offer,
          userIceCandidates: this.localIceCandidates
        });
      })
      .then(peer => {
        this.resetTimeout();
        log(`Created peerconnection ${peer.id}`);
        return new Promise((resolve, reject) => {
          this.peerHandle = SP.peerconnections
            .watch({ id: peer.id })
            .catch(log)
            .on("data", ([doc]) => {
              if (!doc) {
                // Hmm, our candidate is just gone. Annoying and mysterious. Well, start the process
                // over again.
                log(`Peer connection ${peer.id} was deleted, retrying.`);
                this.peerHandle.stop();
                // FIXME: we now just have a promise sitting around? does that matter?
                this.createOffer();
              }
              if (doc.sdpAnswer) {
                this.peerHandle.stop();
                resolve(doc);
              }
            });
        });
      })
      .then(peer => {
        this.resetTimeout();
        log(`Got peer ${peer.id}`);
        this.peerHandle = SP.peerconnections
          .watch({ id: peer.id })
          .on("data", ::this.peerUpdate);
        return this.peerHandle;
      })
      .then(peer => {
        this.resetTimeout();
        return SP.peerconnections.update(this.peer.id, {
          userIceCandidates: this.localIceCandidates
        });
      })
      .catch(err => {
        log(err);
      });
  }

  /**
   * We're the bigger one. Wait for an API object from the target user.
   * @return {[type]} [description]
   */
  waitForOffer() {
    log(`Waiting for offer from ${this.targetUserId}`);
    this.peerHandle = SP.peerconnections.watch({
      userId: this.targetUserId,
      targetUserId: SP.user.id
    });
    return this.peerHandle
      .then(docs => {
        // Hack: for now, delete all the other requests that were here when we got here...
        // This will change once we have a better API notion of what it means to have one user
        // with multiple browsers open
        docs.forEach(doc => {
          SP.peerconnections
            .delete(doc.id)
            .then(() => {
              log(`deleted stale icecandidate ${doc.id}`);
            })
            .catch(log);
        });
        return new Promise((resolve, reject) => {
          this.peerHandle.on("newDoc", data => {
            this.peerHandle.stop();
            resolve(data);
          });
        });
      })
      .then(peer => {
        this.resetTimeout();
        this.peer = peer;
        log(`Got peer ${peer.id}`);
        this.peerHandle = SP.peerconnections
          .watch({ id: this.peer.id })
          .on("data", ::this.peerUpdate);
        return this.setupTurnUrls();
      })
      .catch(log);
  }

  /**
   * Wait on this.peerHandle until we've been assigned turnUrls from the server.
   */
  setupTurnUrls() {
    return new Promise((resolve, reject) => {
      if (this.peer && this.peer.turnUrls && this.peer.turnUrls.length > 1) {
        return resolve(this.peer.turnUrls);
      }
      const onData = ([doc]) => {
        if (!doc) {
          return this.peerHandle.off("data", onData);
        }
        if (!doc.turnUrls || doc.turnUrls.length < 1) {
          return;
        }
        this.peerHandle.off("data", onData);
        resolve(doc.turnUrls);
      };
      this.peerHandle.on("data", onData);
    })
      .then(turnUrls => {
        this.options.configuration.iceServers = turnUrls.map(turnUrl => {
          const { protocol, auth, host } = parseUrl(turnUrl);
          const [username, credential] = auth.split(":");
          return {
            url: `${protocol}${host}`,
            credential: credential,
            username: username
          };
        });
        return this._generateKurentoPeer();
      })
      .then(() => {
        this.peerUpdate([this.peer]);
      });
  }

  /**
   * Code to run every time our salient PeerConnection changes.
   */
  peerUpdate([doc]) {
    this.resetTimeout();
    if (!doc) {
      return this.shutdown();
    }
    this.peer = doc;
    // If we're waiting on their answer and haven't processed it yet...
    if (this.kurentoIsSetup && !this.processedOffer) {
      if (this.isPrimaryUser && this.peer.sdpAnswer) {
        this.processedOffer = true;
        return this.webRtcPeer.processAnswer(
          this.peer.sdpAnswer,
          (err, answer) => {
            const remoteStream = this.webRtcPeer.getRemoteStream();
            log("Remote stream", remoteStream);
            this._streamResolve(remoteStream);
          }
        );
      } else if (!this.isPrimaryUser && this.peer.sdpOffer) {
        this.processedOffer = true;
        return new Promise((resolve, reject) => {
          this.webRtcPeer.processOffer(this.peer.sdpOffer, (err, answer) => {
            if (err) {
              reject(err);
            }
            this._streamResolve(this.webRtcPeer.getRemoteStream());
            resolve(answer);
          });
        })
          .then(answer => {
            this.resetTimeout();
            return SP.peerconnections.update(this.peer.id, {
              sdpAnswer: answer,
              targetUserIceCandidates: this.localIceCandidates
            });
          })
          .catch(err => {
            log(err);
            this.shutdown();
          });
      }
    }
    if (this.kurentoIsSetup) {
      const field = this.isPrimaryUser
        ? "targetUserIceCandidates"
        : "userIceCandidates";
      doc[field] && doc[field].forEach(::this.onRemoteIceCandidate);
    }
  }

  onIceCandidate(candidate) {
    log("ice", candidate);
    this.localIceCandidates.push(candidate);
    // If we're not connected yet, return.
    if (!this.peer) {
      return;
    }
    const field = this.isPrimaryUser
      ? "userIceCandidates"
      : "targetUserIceCandidates";
    SP.peerconnections
      .update(this.peer.id, { [field]: this.localIceCandidates })
      .catch(log);
  }

  onRemoteIceCandidate(candidate) {
    const str = JSON.stringify(candidate);
    if (this.remoteIceCandidatesChecked.indexOf(str) !== -1) {
      // Already checked.
      return;
    }
    log("Evaluating remote ICE candidate", candidate);
    this.remoteIceCandidatesChecked.push(candidate);
    this.webRtcPeer.addIceCandidate(candidate);
  }

  getStream() {
    return this.streamPromise;
  }
}
