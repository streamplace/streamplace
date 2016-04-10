
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
  let readStream;
  let writeStream;

  beforeEach(function() {
    readStream = fs.createReadStream(testFile);
    writeStream = fs.createWriteStream(testFileOut);
  });

  afterEach(function() {
    if (fs.existsSync(testFileOut)) {
      fs.unlinkSync(testFileOut);
    }
  });

  it("should keep streams intact if given no operation", function(done) {
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
    readStream.pipe(mStream);
    mStream.pipe(writeStream);
    writeStream.on("finish", function() {
      expect(packets).to.equal(2872);
      expect(headers).to.equal(1800);
      done();
    });
  });

  // Manually checked from the file
  // First: PTS: 132030, DTS: 126000
  // Last: PTS: 5529060, DTS: 5523030
  // Taking advantage of the fact that PTS is always parsed before DTS here.
  it("should correctly parse PTS and DTS", function(done) {
    let firstPTS;
    let firstDTS;
    let lastPTS;
    let lastDTS;
    const mStream = munger();
    const origReadTimestamp = mStream._readTimestamp;
    mStream._readTimestamp = function(...args) {
      const result = origReadTimestamp.call(mStream, ...args);
      if (!firstPTS) {
        firstPTS = result;
      }
      else if (!firstDTS) {
        firstDTS = result;
      }
      lastPTS = lastDTS;
      lastDTS = result;
    };
    readStream.pipe(mStream);
    mStream.pipe(writeStream);
    writeStream.on("finish", function() {
      expect(firstPTS).to.equal(132030);
      expect(firstDTS).to.equal(126000);
      expect(lastPTS).to.equal(5529060);
      expect(lastDTS).to.equal(5523030);
      done();
    });
  });

  it("should correctly offset PTS and DTS", function(done) {
    let firstPTS;
    let firstDTS;
    let lastPTS;
    let lastDTS;

    const mWriteStream = munger();
    // Never actually do this, lol
    mWriteStream.transformPTS = function(pts) {
      return pts + 1000;
    };
    mWriteStream.transformDTS = function(dts) {
      return dts - 1000;
    };

    const mReadStream = munger();
    const origReadTimestamp = mReadStream._readTimestamp;
    mReadStream._readTimestamp = function(...args) {
      const result = origReadTimestamp.call(mReadStream, ...args);
      if (!firstPTS) {
        firstPTS = result;
      }
      else if (!firstDTS) {
        firstDTS = result;
      }
      lastPTS = lastDTS;
      lastDTS = result;
    };
    readStream.pipe(mWriteStream);
    mWriteStream.pipe(mReadStream);
    mReadStream.pipe(writeStream);
    writeStream.on("finish", function() {
      expect(firstPTS).to.equal(132030 + 1000);
      expect(firstDTS).to.equal(126000 - 1000);
      expect(lastPTS).to.equal(5529060 + 1000);
      expect(lastDTS).to.equal(5523030 - 1000);
      done();
    });
  });
});
