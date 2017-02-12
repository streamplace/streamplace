
/**
 * Pretty short hacky thing to serve keybase-PGP-encrypted subdomains of sp-dev.club
 */

import express from "express";
import {resolve} from "path";
import morgan from "morgan";
import fs from "mz/fs";
import axios from "axios";
// This fella doesn't like being an ES6 package ¯\_(ツ)_/¯
const openpgp = require("openpgp");

const app = express();

app.use(morgan("combined"));
app.use(express.static(resolve(__dirname, "static")));

const port = process.env.PORT || 80;
app.listen(port);
/* eslint-disable no-console */
console.log(`FCFE listening on port ${port}`);
/* eslint-enable no-console */

app.get("/:domain", function(req, res) {
  const split = req.params.domain.split(".");
  if (split.length !== 3) {
    return res.sendStatus(400);
  }
  const domain = split.join(".");
  const [username] = split;

  let cert;
  let key;

  Promise.all([
    fs.readFile(`/certs/${domain}/tls.crt`, "utf8"),
    fs.readFile(`/certs/${domain}/tls.key`, "utf8"),
  ])
  .then(([c, k]) => {
    cert = c;
    key = k;
    return axios.get(`https://keybase.io/_/api/1.0/user/lookup.json?usernames=${username}`);
  })
  .catch((err) => {
    if (err !== "ENOENT") {
      res.sendStatus(500);
      throw err;
    }
    // Ok, we don't have theirs, send 'em a 404
    res.sendStatus(404);
  })
  .then((response) => {
    const data = response.data;
    if (data.status.code !== 0) {
      res.status(400);
      return res.end(`Bad response from keybase: ${JSON.stringify(data.status, null, 4)}`);
    }
    const publicKey = data.them[0].public_keys.primary.bundle;
    if (!key) {
      res.status(412);
      return res.end("Couldn't find a public PGP key on your keybase profile.");
    }
    return openpgp.encrypt({
      data: JSON.stringify({cert, key}),
      publicKeys: openpgp.key.readArmored(publicKey).keys,
    });
  })
  .then((cyphertext) => {
    res.status(200).end(cyphertext.data);
  })
  .catch((err) => {
    res.sendStatus(500);
    throw err;
  });

});

process.on("SIGTERM", function () {
  process.exit(0);
});
