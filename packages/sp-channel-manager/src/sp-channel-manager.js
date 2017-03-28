
/**
 * This guy will eventually run probably as a vertex, one per node.
 */

import winston from "winston";
import _ from "underscore";
import SP from "sp-client";
import request from "request-promise";
import getMyIp from "./get-my-ip";

export default class ChannelManager {
  constructor() {
    getMyIp().then((ip) => {
      winston.info(`ChannelManager booting up. My IP is ${ip}`);
      this.myIp = ip;
      this.channelHandle = SP.channels.watch({})
      .on("data", (channels) => {
        this.channels = channels;
        this.reconcile();
      });
      return this.channelHandle;
    })
    .catch(::winston.error);
  }

  cleanup() {
    this.channelHandle.stop();
  }

  reconcile() {
    this.channels.forEach(::this.reconcileChannel);
  }

  reconcileChannel(channel) {
    if (channel.turnUrl !== this.myIp) {
      SP.channels.update(channel.id, {turnUrl: this.myIp}).then(() => {
        winston.info(`Updated turnUrl for channel "${channel.slug}" (${channel.id})`);
      })
      .catch(::winston.error);
    }
  }
}

if (!module.parent) {
  SP.connect()
  .then(() => {
    new ChannelManager();
  })
  .catch((err) => {
    winston.error(err);
    process.exit(1);
  });
}
