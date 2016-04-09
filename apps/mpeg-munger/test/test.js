
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

  // Gonna do as much monkeypatching for testing as I can, so that every single line of code in
  // the live-video code path is as lean as possible.
  it("should get an accurate packet count", function(done) {
    let packets = 0;
    let headers = 0;
    const mStream = munger();
    const origRewrite = mStream._rewrite;
    const origRewriteHeader = mStream._rewriteHeader;
    mStream._rewrite = function(...args) {
      packets += 1;
      return origRewrite.call(mStream, ...args);
    };
    mStream._rewriteHeader = function(...args) {
      headers += 1;
      return origRewriteHeader.call(mStream, ...args);
    };
    const readStream = fs.createReadStream(testFile);
    const writeStream = fs.createWriteStream(testFileOut);
    readStream.pipe(mStream);
    mStream.pipe(writeStream);
    writeStream.on("finish", function() {
      expect(packets).to.equal(2872);
      expect(headers).to.equal(1800);
      done();
    });
  });
});
