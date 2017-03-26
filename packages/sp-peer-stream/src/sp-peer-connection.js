
/**
 * Wrapper for Kurento's PeerConnection, which in turn wraps and abstracts the actual WebRTC
 * interfaces.
 */

import debug from "debug";
import SP from "sp-client";
import EE from "wolfy87-eventemitter";
import KurentoUtils from "kurento-utils";

if (typeof RTCPeerConnection === "undefined") {
  if (typeof webkitRTCPeerConnection !== "undefined") {
    window.RTCPeerConnection = window.webkitRTCPeerConnection;
  }
  else if (typeof mozRTCPeerConnection !== "undefined") {
    window.RTCPeerConnection = window.mozRTCPeerConnection;
  }
  else {
    SP.error("No RTCPeerConnection implementation found");
  }
}

const TURN_URL = window.location.hostname;

const log = debug("sp:sp-peer-connection");

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
  "isolationchange",
];

export default class SPPeerConnection extends EE {
  constructor({targetUserId, stream}) {
    super();
    this.isShutDown = false;
    this.targetUserId = targetUserId;
    this.videoStream = stream;
    this.localIceCandidates = [];
    this.remoteIceCandidatesChecked = [];
    this.processedOffer = false;
    this.options = {
      localVideo: stream,
      mediaConstraints: {
        audio: true,
        video: {
          width: 1280,
          height: 720
        }
      },
      onicecandidate : ::this.onIceCandidate,
      configuration: {
        iceServers : [{
          "url": `turn:${TURN_URL}`,
          "credential":"streamplace",
          "username":"streamplace"
        }],
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
    }
    else {
      this.waitForOffer();
    }
  }

  shutdown() {
    if (this.isShutDown === true) {
      return;
    }
    this.webRtcPeer.dispose();
    this.isShutDown = true;
    this.peerHandle && this.peerHandle.stop();
    this.emit("disconnected");
  }

  _generateKurentoPeer() {
    return new Promise((resolve, reject) => {
      log("Creating Kurento peer");
      this.webRtcPeer = KurentoUtils.WebRtcPeer.WebRtcPeerSendrecv(this.options, (error) => {
        this.webRtcPeer.peerConnection.addEventListener("iceconnectionstatechange", ::this.handleConnectionChange);
        if (error) {
          return reject(error);
        }
        resolve();
      });
    });
  }

  handleConnectionChange(e) {
    const state = e.currentTarget.iceConnectionState;
    log(`ICE Connection State: ${state}`);
    if (state === "completed") {
      // Cool, we're connected, no reason to keep that peerconnection around anymore
      this.peerHandle.stop();
      if (this.isPrimaryUser) {
        SP.peerconnections.delete(this.peer.id).catch(log);
      }
    }
    if (state === "failed" || state === "disconnected" || state === "closed") {
      this.shutdown();
    }
  }

  /**
   * We're the smaller one. Create an API object for it.
   */
  createOffer() {
    return this._generateKurentoPeer()
    .then(() => {
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
    .then((offer) => {
      log("Creating peerconnection");
      return SP.peerconnections.create({
        userId: SP.user.id,
        targetUserId: this.targetUserId,
        sdpOffer: offer,
        userIceCandidates: this.localIceCandidates,
      });
    })
    .then((ice) => {
      log(`Created peerconnection ${ice.id}`);
      return new Promise((resolve, reject) => {
        const handle = SP.peerconnections.watch({id: ice.id})
        .catch(log)
        .on("data", ([doc]) => {
          if (!doc) {
            // Hmm, our candidate is just gone. Annoying and mysterious. Well, start the process
            // over again.
            log(`Peer connection ${ice.id} was deleted, retrying.`);
            handle.stop();
            // FIXME: we now just have a promise sitting around? does that matter?
            this.createOffer();
          }
          if (doc.sdpAnswer) {
            handle.stop();
            resolve(doc);
          }
        });
      });
    })
    .then((peer) => {
      log(`Got peer ${peer.id}`);
      this.peerHandle = SP.peerconnections.watch({id: peer.id}).on("data", ::this.peerUpdate);
      return this.peerHandle;
    })
    .then((peer) => {
      return SP.peerconnections.update(this.peer.id, {userIceCandidates: this.localIceCandidates});
    })
    .catch((err) => {
      log(err);
    });
  }

  /**
   * We're the bigger one. Wait for an API object from the target user.
   * @return {[type]} [description]
   */
  waitForOffer() {
    let handle;
    this._generateKurentoPeer()
    .then(() => {
      log(`Waiting for offer from ${this.targetUserId}`);
      handle = SP.peerconnections.watch({
        userId: this.targetUserId,
        targetUserId: SP.user.id
      });
      return handle;
    })
    .then((docs) => {
      // Hack: for now, delete all the other requests that were here when we got here...
      // This will change once we have a better API notion of what it means to have one user
      // with multiple browsers open
      docs.forEach((doc) => {
        SP.peerconnections.delete(doc.id)
        .then(() => {
          log(`deleted stale icecandidate ${doc.id}`);
        })
        .catch(log);
      });
      return new Promise((resolve, reject) => {
        handle.on("newDoc", (data) => {
          handle.stop();
          resolve(data);
        });
      });
    })
    .then((peer) => {
      log(`Got peer ${peer.id}`);
      this.peerHandle = SP.peerconnections.watch({id: peer.id}).on("data", ::this.peerUpdate);
      return this.peerHandle;
    })
    .then((peer) => {

    })
    .then(() => {
      return new Promise((resolve, reject) => {
        this.webRtcPeer.processOffer(this.peer.sdpOffer, (err, answer) => {
          if (err) {
            reject(err);
          }
          this._streamResolve(this.webRtcPeer.getRemoteStream());
          resolve(answer);
        });
      });
    })
    .then((answer) => {
      return SP.peerconnections.update(this.peer.id, {
        sdpAnswer: answer,
        targetUserIceCandidates: this.localIceCandidates
      });
    })
    .catch(log);
  }

  peerUpdate([doc]) {
    log("update", doc);
    this.peer = doc;
    // If we're waiting on their answer and haven't processed it yet...
    if (this.isPrimaryUser && !this.processedOffer && this.peer.sdpAnswer) {
      this.processedOffer = true;
      this.webRtcPeer.processAnswer(this.peer.sdpAnswer, (err, answer) => {
        const remoteStream = this.webRtcPeer.getRemoteStream();
        log("Remote stream", remoteStream);
        this._streamResolve(remoteStream);
      });
    }
    const field = this.isPrimaryUser ? "targetUserIceCandidates" : "userIceCandidates";
    doc[field] && doc[field].forEach(::this.onRemoteIceCandidate);
  }

  onIceCandidate(candidate) {
    log("ice", candidate);
    this.localIceCandidates.push(candidate);
    // If we're not connected yet, return.
    if (!this.peer) {
      return;
    }
    const field = this.isPrimaryUser ? "userIceCandidates" : "targetUserIceCandidates";
    SP.peerconnections.update(this.peer.id, {[field]: this.localIceCandidates}).catch(log);
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
