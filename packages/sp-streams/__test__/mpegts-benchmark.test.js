import socketIngressStream from "../src/socket-ingress-stream";
import ffmpeg from "../src/ffmpeg";

describe("fast mpegts", () => {
  it("should import stuff", () => {
    expect(socketIngressStream).toBeDefined();
  });

  // it("should do things in a reasonable time", done => {});
});
