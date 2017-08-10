import config from "../src/sp-configuration";
import { resolve } from "path";

describe("fromValuesFile", () => {
  it("should properly parse a Helm file into our environment variables", () => {
    config.loadValuesFile(resolve(__dirname, "dummy-values.yaml"));
    expect(config.require("TOP_LEVEL")).toBe("i am topLevel");
    expect(config.require("GLOBAL_VALUE")).toBe("i am globalValue");
    expect(config.require("NOT_GLOBAL_SUB_KEY")).toBe("i am subKey");
  });
});
