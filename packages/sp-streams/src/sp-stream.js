import debug from "debug";
import fs from "fs-extra";
import constantFpsStream from "./constant-fps-stream";
import dashStream, { MANIFEST_NAME } from "./dash-stream";
import dashServer from "./dash-server";

const log = debug("sp:sp-stream");

/* eslint-disable no-console */
/**
 * TODO: I want this to have a really really polymorphic interface that's usable in a wide variety
 * of cases.
 */
export default async function spStream({ filePath }) {
  if (!await fs.pathExists(filePath)) {
    throw new Error(`File not found: ${filePath}`);
  }
  const file = fs.createReadStream(filePath);
  const constantFps = constantFpsStream({ fps: 30 });
  const dash = dashStream();
  file.pipe(constantFps);
  constantFps.pipe(dash);
  const server = dashServer(dash);
  console.log("just a sec...");
  let listener;
  const portProm = new Promise((resolve, reject) => {
    const listener = server.listen(0, err => {
      if (err) return reject(err);
      resolve(listener.address().port);
    });
  });
  await new Promise((resolve, reject) => {
    dash.once("data", resolve);
    dash.on("error", reject);
  });
  const port = await portProm;
  console.log(`Streaming to http://localhost:${port}/${MANIFEST_NAME}`);
  dash.on("end", () => console.log("end"));
}
