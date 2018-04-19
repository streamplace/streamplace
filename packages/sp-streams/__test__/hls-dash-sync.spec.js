import fs from "fs";
import resolve from "path";
import { hlsFix } from "../src/hls-dash-sync";
import path from "path";

describe("hlsFix", () => {
  it("should reformat our hls manifests", () => {
    const dir = path.resolve(__dirname, "hls-dash-sync");
    const hls = fs.readFileSync(path.resolve(dir, "hls-input.m3u8"), "utf8");
    const dash = fs.readFileSync(path.resolve(dir, "dash-input.mpd"), "utf8");
    const hlsOutput = fs.readFileSync(
      path.resolve(dir, "hls-output.m3u8"),
      "utf8"
    );
    expect(hlsFix({ hls, dash })).toEqual(hlsOutput);
  });
});
