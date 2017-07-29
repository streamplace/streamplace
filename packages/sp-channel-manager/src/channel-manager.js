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
    getMyIp()
      .then(ip => {
        this.activeChannels = {};
        winston.info(`ChannelManager booting up. My IP is ${ip}`);
        this.myIp = ip;
        this.properTurnUrls = [`turn://${CREDENTIALS}@${this.myIp}`];
        this.peerHandle = SP.peerconnections
          .watch({})
          .on("data", peerConnections => {
            this.peerConnections = peerConnections;
            this.reconcilePeers();
          });
        return this.peerHandle;
      })
      .then(() => {
        this.channelHandle = SP.channels.watch({}).on("data", channels => {
          this.channels = channels;
          this.reconcileChannels();
        });
      })
      .catch(err => {
        winston.error(err);
        process.exit(1);
      });
  }

  cleanup() {
    this.peerHandle.stop();
    this.channelHandle.stop();
  }

  reconcilePeers() {
    winston.info(`Got ${this.peerConnections.length} peers.`);
    this.peerConnections.forEach(::this.reconcilePeer);
  }

  reconcileChannels() {
    winston.info(`Got ${this.channels.length} channels.`);
    this.channels.forEach(::this.reconcileChannel);
  }

  /**
   * Set up a channel real nice
   */
  reconcileChannel(channel) {
    if (!channel.activeSceneId) {
      winston.info(`${channel.slug} has no scene, creating`);
      SP.scenes
        .create({
          userId: channel.userId,
          channelId: channel.id,
          title: `${channel.slug} scene`,
          width: 1920,
          height: 1080,
          children: []
        })
        .then(scene => {
          winston.info(`Created scene ${scene.id} for channel ${channel.slug}`);
          return SP.channels.update(channel.id, { activeSceneId: scene.id });
        })
        .catch(err => {
          winston.error(`Error creating scene for ${channel.slug}`);
          winston.error(err);
        });
    }
  }

  reconcilePeer(peer) {
    if (!_(peer.turnUrls).isEqual(this.properTurnUrls)) {
      SP.peerconnections
        .update(peer.id, {
          turnUrls: this.properTurnUrls
        })
        .then(() => {
          winston.info(`Added TURN URLs to PeerConnection ${peer.id}`);
        })
        .catch(err => {
          winston.error(err);
        });
    }
  }
}
