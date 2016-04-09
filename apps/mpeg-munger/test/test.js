
import expect from "expect.js";
import fs from "fs";
import path from "path";

import munger from "../src/munger.js";

const testFile = path.resolve(__dirname, "testvid.ts");
const testFileOut = path.resolve(__dirname, "testvid.out.ts");

describe("mpeg-munger", function(done) {
  afterEach(function() {
    if (fs.existsSync(testFileOut)) {
      fs.unlinkSync(testFileOut);
    }
  });

  it("should not modify file size", function(done) {
    const readStream = fs.createReadStream(testFile);
    const writeStream = fs.createWriteStream(testFileOut);
    const mStream = munger();
    readStream.pipe(mStream);
    mStream.pipe(writeStream);
    readStream.on("end", function() {
      // Wait for the last IO to run, then...
      setImmediate(function() {
        const inStats = fs.statSync(testFile);
        const outStats = fs.statSync(testFileOut);
        expect(inStats.size).to.equal(outStats.size);
        done();
      });
    });
    // mStream.pipe(writeStream);
  });
});
