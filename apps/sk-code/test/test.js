
import SKCode from "../src/sk-code.js";
import expect from "expect.js";

describe("SKCode", function() {
  this.slow(0);
  it("should create", function() {
    let code = new SKCode();
  });

  it("should toColors()", function() {
    let code = new SKCode();
    code.toColors().forEach((cell) => {
      cell.forEach((color) => {
        if (!(color === 0 || color === 255)) {
          expect.fail("Got a color that's not 0 or 255!");
        }
      });
    });
  });

  const dumpColors = function(cells) {
    let ret = "";
    cells.forEach((cell) => {
      let line = [];
      cell.forEach((color) => {
        if (color === 0) {
          line.push("000");
          return;
        }
        line.push(`${color}`);
      });
      ret += `${line.join(" - ")}\n`;
    });
    // console.log(ret);
    return ret;
  };

  const tryTS = function(ts) {
    const code = new SKCode(ts);
    const colors = code.toColors();
    dumpColors(colors);
    const code2 = SKCode.fromColors(colors);
    const colors2 = code2.toColors();
    expect(dumpColors(colors2)).to.equal(dumpColors(colors));
    expect(code2.value).to.equal(ts);
  };

  it("should correctly toColors and fromColors every timestamp", function() {
    tryTS(0);
    tryTS(86400000);
  });
});
