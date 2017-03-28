
/**
 * This guy will eventually run probably as a vertex, one per node.
 */

import winston from "winston";
import _ from "underscore";
import SP from "sp-client";
import request from "request-promise";
import getMyIp from "./get-my-ip";

// These credentials are hardcoded for now, but eventually I'll be in charge of creating them and
// interfacing with Coturn. Cool!
const CREDENTIALS = "streamplace:streamplace";

export default class ChannelManager {
  constructor() {
    getMyIp().then((ip) => {
      winston.info(`ChannelManager booting up. My IP is ${ip}`);
      this.myIp = ip;
      this.properTurnUrls = [
        `turn://${CREDENTIALS}@${this.myIp}`
      ];
      this.peerHandle = SP.peerconnections.watch({})
      .on("data", (peerConnections) => {
        this.peerConnections = peerConnections;
        this.reconcile();
      })
      .catch((err) => {
        winston.error(err);
        process.exit(1);
      });
      return this.peerHandle;
    })
    .catch(::winston.error);
  }

  cleanup() {
    this.peerHandle.stop();
  }

  reconcile() {
    winston.info(`Got ${this.peerConnections.length} peers.`);
    this.peerConnections.forEach(::this.reconcilePeer);
  }

  reconcilePeer(peer) {
    if (!_(peer.turnUrls).isEqual(this.properTurnUrls)) {
      SP.peerconnections.update(peer.id, {
        turnUrls: this.properTurnUrls,
      })
      .then(() => {
        winston.info(`Added TURN URLs to PeerConnection ${peer.id}`);
      })
      .catch((err) => {
        winston.error(err);
      });
    }
    // if (channel.turnUrl !== this.myIp) {
    //   SP.channels.update(channel.id, {turnUrl: this.myIp}).then(() => {
    //     winston.info(`Updated turnUrl for channel "${channel.slug}" (${channel.id})`);
    //   })
    //   .catch(::winston.error);
    // }
  }
}

if (!module.parent) {
  SP.on("error", (err) => {
    winston.error(err);
  });
  SP.connect()
  .then(() => {
    new ChannelManager();
  })
  .catch((err) => {
    winston.error(err);
    process.exit(1);
  });
}
