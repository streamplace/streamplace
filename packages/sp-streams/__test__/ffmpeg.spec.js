import { framehash } from "../src/ffmpeg";
import path from "path";

const TEST_FILE_PATH = path.resolve(__dirname, "elis-face.ts");

describe("framehash", () => {
  it(
    "should get a framehash",
    () => {
      return framehash(TEST_FILE_PATH).then(hash => {
        expect(hash).toEqual(
          "745537cea12860e9b0bbf8f7fabdfeadea4080210de1e64257eb70e97f8f8ac5"
        );
      });
    },
    10000
  );
});
