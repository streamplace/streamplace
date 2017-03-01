
/**
 * Pretty short hacky thing to serve keybase-PGP-encrypted subdomains of sp-dev.club
 */

import express from "express";
import {resolve} from "path";
import morgan from "morgan";
import fs from "mz/fs";
import axios from "axios";
import winston from "winston";
// This fella doesn't like being an ES6 package ¯\_(ツ)_/¯
const openpgp = require("openpgp");

const app = express();

app.use(morgan("combined"));
app.use(express.static(resolve(__dirname, "static")));

const port = process.env.PORT || 80;
app.listen(port);

winston.info(`FCFE listening on port ${port}`);

const getCertFiles = function(domain) {
  return Promise.all([
    fs.readFile(`/certs/${domain}/tls.crt`, "utf8"),
    fs.readFile(`/certs/${domain}/tls.key`, "utf8"),
  ])
  .catch((err) => {
    if (err.code !== "ENOENT") {
      throw err;
    }
    const suffix = domain.split(".").slice(1).join(".");
    return Promise.all([
      fs.readFile(`/certs/wild-card.${suffix}/tls.crt`, "utf8"),
      fs.readFile(`/certs/wild-card.${suffix}/tls.key`, "utf8"),
    ]);
  });
};

app.get("/:domain", function(req, res) {
  const split = req.params.domain.split(".");
  if (split.length !== 3) {
    return res.sendStatus(400);
  }
  const domain = split.join(".");
  const [username] = split;

  let cert;
  let key;

  getCertFiles(domain)
  .then(([c, k]) => {
    cert = c;
    key = k;
    return axios.get(`https://keybase.io/_/api/1.0/user/lookup.json?usernames=${username}`);
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
    winston.error(err);
    if (err.code === "ENOENT") {
      res.sendStatus(404);
    }
    else {
      res.sendStatus(500);
    }
  });

});

process.on("SIGTERM", function () {
  process.exit(0);
});
