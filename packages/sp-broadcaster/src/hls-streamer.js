import SP from "sp-client";
import winston from "winston";
import config from "sp-configuration";
import FileStreamManager from "./file-stream-manager";
import { tcpInputStream, hlsOuputStream } from "sp-streams";

export default class HLSStreamer {
  constructor({ broadcastId, podIp }) {
    winston.info(
      `HLSStreamer booting up for ${JSON.stringify({ broadcastId, podIp })}`
    );
    this.broadcastId = broadcastId;
    this.podIp = podIp;
    this.managers = {};

    SP.broadcasts
      .watch({ id: broadcastId })
      .on("data", ([broadcast]) => {
        if (!broadcast) {
          winston.warn(
            `Looks like broadcast ${broadcastId} was deleted, holding...`
          );
          return;
        }
        broadcast.sources.filter(s => s.kind === "File").forEach(source => {
          if (this.managers[source.id]) {
            // We're already streamin' this one. Great.
            return;
          }
          this.managers[source.id] = new FileStreamManager({
            fileId: source.id
          });
        });
      })
      .on("deletedDoc", () => {
        this.managers.forEach(m => m.shutdown());
      });
  }

  playVideo() {}
}
