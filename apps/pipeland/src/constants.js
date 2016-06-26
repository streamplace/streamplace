
import os from "os";
import config from "sk-config";
import winston from "winston";

export const SERVER_START_TIME = (new Date()).getTime();
export const TIME_BASE = 90; // We'll obviously have to support more than this in the future.
export const PTS_OFFSET_RESET = 500/*ms*/ * TIME_BASE;

// Let's get my IP address. If it's Docker we're localhost for now, if it's Kubernetes it's our
// pod IP.

const PLATFORM = config.require("PLATFORM");
let ip;
if (PLATFORM === "docker") {
  winston.info("Platform is Docker -- for now, that means our IP is localhost.");
  ip = "127.0.0.1";
}
else if (PLATFORM === "kubernetes") {
  const ifaces = os.networkInterfaces();
  const ips = [];

  Object.keys(ifaces).forEach((ifname) => {
    ifaces[ifname].forEach(function (iface) {
      if ("IPv4" !== iface.family || iface.internal !== false) {
        // skip over internal (i.e. 127.0.0.1) and non-ipv4 addresses
        return;
      }
      ips.push(iface.address);
    });
  });
  if (ips.length === 0) {
    winston.error("Couldn't find non-localhost IP! Are you sure that PLATFORM is correctly set to Kubernetes?");
    throw new Error("No non-localhost IP found");
  }
  if (ips.length > 1) {
    winston.warn("Hmmm, this pod appears to have more than one IP address. That's... weird. We're going to use the first one, might be fine, just wanted to let you know.");
    winston.warn("IPs: ", ips.join(", "));
  }
  ip = ips[0];
}

export const MY_IP = ip;
