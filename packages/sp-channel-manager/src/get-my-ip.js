
// i wonder how much of the internet breaks if icanhazip.com stops working? if this file scares
// you and you want to write some fallbacks, be my guest.

import request from "request-promise";
import config from "sp-configuration";

const DEV_EXTERNAL_IP = config.optional("DEV_EXTERNAL_IP");

const prom = request("https://icanhazip.com").then((data) => {
  return data.replace(/\s/g, "");
});

export default function getMyIp() {
  if (DEV_EXTERNAL_IP) {
    return Promise.resolve(DEV_EXTERNAL_IP);
  }
  return prom;
}
