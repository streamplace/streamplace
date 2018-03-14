import colorbarsStream from "../src/colorbars-stream";
import { killall } from "../src/ffmpeg";

describe("colorbarsStream", () => {
  afterEach(() => {
    killall();
  });

  it("should produce something", () => {
    const colorbars = colorbarsStream();
    expect(colorbars).toBeDefined();
  });

  it(
    "should produce some kind of data",
    done => {
      const colorbars = colorbarsStream();
      colorbars.once("data", () => {
        done();
      });
    },
    10000
  );
});
