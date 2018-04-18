import debug from "debug";
import fs from "fs-extra";
import constantFpsStream from "./constant-fps-stream";
import ptsNormalizerStream from "./pts-normalizer-stream";
import dashStream, { MANIFEST_NAME } from "./dash-stream";
import dashServer from "./dash-server";

const log = debug("sp:sp-stream");

/**
 * TODO: I want this to have a really really polymorphic interface that's usable in a wide variety
 * of cases.
 */
export default async function spStream({ filePath, loop }) {
  if (!(await fs.pathExists(filePath))) {
    throw new Error(`File not found: ${filePath}`);
  }
  const constantFps = constantFpsStream({ fps: 30 });
  const dash = dashStream();

  if (loop) {
    const ptsNormalizer = ptsNormalizerStream();
    ptsNormalizer.pipe(constantFps);
    const restart = () => {
      debug("restarting");
      const file = fs.createReadStream(filePath);
      file.pipe(ptsNormalizer, { end: false });
      file.on("end", restart);
    };
    restart();
  } else {
    const file = fs.createReadStream(filePath);
    file.pipe(constantFps);
  }
  constantFps.pipe(dash);
  const server = dashServer(dash);
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
  dash.url = `http://localhost:${port}/${MANIFEST_NAME}`;
  return dash;
}
