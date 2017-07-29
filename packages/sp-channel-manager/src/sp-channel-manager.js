/**
 * This is kind of a lie, I do a lot more than just manage channels these days. The actuall channel
 * manging happens over at channel-manager.js -- I'm the main file that boots up all our managers
 * and schedulers and stuff.
 */

import winston from "winston";
import SP from "sp-client";
import express from "express";
import ChannelManager from "./channel-manager";
import BroadcastScheduler from "./broadcast-scheduler";

// Health check
const app = express();
app.get("/healthz", (req, res) => {
  res.sendStatus(200);
});
app.listen(80);

if (!module.parent) {
  SP.on("error", err => {
    winston.error(err);
  });
  SP.connect()
    .then(() => {
      new ChannelManager();
      new BroadcastScheduler();
    })
    .catch(err => {
      winston.error(err);
      process.exit(1);
    });
}
