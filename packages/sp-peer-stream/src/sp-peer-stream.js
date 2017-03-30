
/**
 * This is the external interfaces that folks use to set up WebRTCPeerConnections.
 */

import SPPeer from "./sp-peer";
import SPPeerConnection from "./sp-peer-connection";

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

/**
 * We learned about an new TURN url available to us! How splendid. Add it to our list that we'll
 * use to try and connect to people.
 *
 * Streamplace doesn't fuck with STUN URLs. We don't have to. TURN takes case of it.
 */
export function addTurnUrl(turnUrl) {
  SPPeerConnection.addTurnUrl(turnUrl);
}
