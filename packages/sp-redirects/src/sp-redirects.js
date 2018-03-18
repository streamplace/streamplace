import express from "express";
import morgan from "morgan";
import { safeLoad as parseYAML } from "js-yaml";
import axios from "axios";
import https from "https";
import url from "url";

/* eslint-disable no-console */

const app = express();
app.use(morgan("dev"));

const UPDATE_URL = "https://s3-us-west-2.amazonaws.com/crap.stream.place/apps";
const updateInfo = url.parse(UPDATE_URL);
console.log(updateInfo);
const WINDOWS_UPDATE_URL = `${UPDATE_URL}/latest.yml`;
const MAC_UPDATE_URL = `${UPDATE_URL}/latest-mac.yml`;
const WINDOWS_PATH = "/Streamplace%20Setup.exe";
const MAC_PATH = "/Streamplace.dmg";

const listener = app.listen(process.env.PORT || 80, () => {
  console.log(`sp-redirects listening on ${listener.address().port}`);
});

let windowsUpdateData;
let macUpdateData;

app.get("/healthz", (req, res) => res.sendStatus(200));

const doProxy = ({ myPath, updateUrl }) => {
  app.get(myPath, (req, res, next) => {
    axios
      .get(updateUrl)
      .then(updateResponse => {
        windowsUpdateData = parseYAML(updateResponse.data);
        const updateReq = https.request(
          {
            host: updateInfo.host,
            protocol: updateInfo.protocol,
            path: `${updateInfo.path}/${encodeURIComponent(
              windowsUpdateData.path
            )}`
          },
          updateRes => {
            res.set("content-type", updateRes.headers["content-type"]);
            res.set("content-length", updateRes.headers["content-length"]);
            res.set("last-modified", updateRes.headers["last-modified"]);
            updateRes.pipe(res);
          }
        );
        updateReq.end();
      })
      .catch(err => {
        console.error(err);
        res.sendStatus(500);
      });
  });
};
doProxy({
  myPath: WINDOWS_PATH,
  updateUrl: WINDOWS_UPDATE_URL
});
doProxy({
  myPath: MAC_PATH,
  updateUrl: MAC_UPDATE_URL
});
