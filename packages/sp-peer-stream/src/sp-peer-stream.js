
/**
 * This is the external interfaces that folks use to set up WebRTCPeerConnections.
 */

import SPPeer from "./sp-peer";

const peers = {};

/**
 * Goes and talks to a peer. If we have a stream for them, resolves right away. Otherwise resolves
 * to a peer once we have one.
 */
export function getPeer(userId) {
  if (!peers[userId]) {
    peers[userId] = new SPPeer(userId);
  }

  return peers[userId];
}
