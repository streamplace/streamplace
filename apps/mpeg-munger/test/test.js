
import expect from "expect.js";
import fs from "fs";
import path from "path";
import crypto from "crypto";

import munger from "../src/munger.js";

const hashFile = function(filename) {
  const data = fs.readFileSync(filename);
  return crypto.createHmac("sha256", "").update(data).digest("hex");
};

const testForFile = function(fileDetails) {
  const testFile = path.resolve(__dirname, fileDetails.name);
  const testFileOut = path.resolve(__dirname, fileDetails.name + ".out");
  describe(fileDetails.name, function(done) {
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
        expect(packets).to.equal(fileDetails.packets);
        expect(headers).to.equal(fileDetails.headers);
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
        expect(firstPTS).to.equal(fileDetails.firstPTS);
        expect(firstDTS).to.equal(fileDetails.firstDTS);
        expect(lastPTS).to.equal(fileDetails.lastPTS);
        expect(lastDTS).to.equal(fileDetails.lastDTS);
        done();
      });
    });

    it("should correctly offset PTS and DTS", function(done) {
      let firstPTS;
      let firstDTS = 0;
      let lastPTS;
      let lastDTS = 0;

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
        expect(firstPTS).to.equal(fileDetails.firstPTS + 1000);
        expect(firstDTS).to.equal(fileDetails.firstDTS - 1000);
        expect(lastPTS).to.equal(fileDetails.lastPTS + 1000);
        expect(lastDTS).to.equal(fileDetails.lastDTS - 1000);
        done();
      });
    });
  });
};

testForFile({
  name: "testvid.ts",
  packets: 2872,
  headers: 1800,
  firstPTS: 132030,
  lastPTS: 5529060,
  firstDTS: 126000,
  lastDTS: 5523030
});

testForFile({
  name: "elis-face.ts",
  packets: 29900,
  headers: 1208,
  firstPTS: 126000,
  lastPTS: 2826017,
  firstDTS: 0,
  lastDTS: 0
});
