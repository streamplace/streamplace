
import expect from "expect.js";
import fs from "fs";
import path from "path";
import crypto from "crypto";

import munger from "../src/munger.js";

const testFile = path.resolve(__dirname, "testvid.ts");
const testFileOut = path.resolve(__dirname, "testvid.out.ts");

const hashFile = function(filename) {
  const data = fs.readFileSync(filename);
  return crypto.createHmac("sha256", "").update(data).digest("hex");
};

describe("mpeg-munger", function(done) {
  afterEach(function() {
    if (fs.existsSync(testFileOut)) {
      fs.unlinkSync(testFileOut);
    }
  });

  it("should keep streams intact if given no operation", function(done) {
    const readStream = fs.createReadStream(testFile);
    const writeStream = fs.createWriteStream(testFileOut);
    const mStream = munger();
    readStream.pipe(mStream);
    mStream.pipe(writeStream);
    writeStream.on("finish", function() {
      const inStats = fs.statSync(testFile);
      const outStats = fs.statSync(testFileOut);
      expect(inStats.size).to.equal(outStats.size);
      expect(hashFile(testFile)).to.equal(hashFile(testFileOut));
      done();
    });
  });
});
