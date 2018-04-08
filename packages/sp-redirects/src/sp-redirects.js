import express from "express";
import morgan from "morgan";
import { safeLoad as parseYAML } from "js-yaml";
import axios from "axios";
import https from "https";
import url from "url";

/* eslint-disable no-console */

const app = express();

// Short-circuit /healthz so it doesn't log
app.use((req, res, next) => {
  if (req.url === "/healthz") {
    return res.sendStatus(200);
  }
  return next();
});

process.on("SIGTERM", function() {
  process.exit(0);
});

app.use(morgan("dev"));

const UPDATE_URL = "https://s3-us-west-2.amazonaws.com/crap.stream.place/apps";
const updateInfo = url.parse(UPDATE_URL);
const WINDOWS_UPDATE_URL = `${UPDATE_URL}/latest.yml`;
const MAC_UPDATE_URL = `${UPDATE_URL}/latest-mac.yml`;
const WINDOWS_PATH = "/Streamplace%20Setup.exe";
const MAC_PATH = "/Streamplace.dmg";

const listener = app.listen(process.env.PORT || 80, () => {
  console.log(`sp-redirects listening on ${listener.address().port}`);
});

const doProxy = ({ myPath, updateUrl, mapDataToFile }) => {
  app.get(myPath, (req, res, next) => {
    axios
      .get(updateUrl)
      .then(updateResponse => {
        const updateData = parseYAML(updateResponse.data);
        const filePath = mapDataToFile(updateData);

        // Uncomment this and comment the rest of the function if you wanna redirect
        // res.redirect(`${updateInfo.href}/${encodeURIComponent(filePath)}`);
        const updateReq = https.request(
          {
            host: updateInfo.host,
            protocol: updateInfo.protocol,
            path: `${updateInfo.path}/${encodeURIComponent(filePath)}`
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
  updateUrl: WINDOWS_UPDATE_URL,
  mapDataToFile: data => data.path
});
doProxy({
  myPath: MAC_PATH,
  updateUrl: MAC_UPDATE_URL,
  mapDataToFile: data =>
    data.files.filter(file => file.url.endsWith(".dmg"))[0].url
});
