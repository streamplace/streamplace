import mpegMungerStream, { streamIsVideo } from "./mpeg-munger-stream";

/**
 * Takes in a mpegts stream and emits e.g. 30 frames per real-time second. Like
 * ffmpeg's "-re" flag.
 * @param {{fps}} fps
 */
export default function constantFpsStream({ fps } = {}) {
  if (typeof fps !== "number") {
    throw new Error(`${fps} is not a valid fps`);
  }
  const mpegMunger = mpegMungerStream();
  let frames = 0;
  let firstFrameTime;
  mpegMunger.on("pts", ({ pts, streamId }) => {
    if (!streamIsVideo(streamId)) {
      return;
    }
    if (!firstFrameTime) {
      firstFrameTime = Date.now();
    }
    frames += 1;
    check();
  });
  const check = () => {
    if (firstFrameTime === undefined) {
      return;
    }
    const expectedFrames = Math.ceil(
      (Date.now() - firstFrameTime) / 1000 * fps
    );
    // console.log(`Expected ${expectedFrames}, found ${frames}`);
    if (expectedFrames > frames) {
      mpegMunger.resume();
    } else if (expectedFrames < frames) {
      mpegMunger.pause();
    }
  };
  const interval = setInterval(check, 250);
  mpegMunger.on("end", () => {
    clearInterval(interval);
  });
  return mpegMunger;
}
