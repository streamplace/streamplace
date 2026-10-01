import { agree, shouldCommit } from "streamplace";
import { describe, expect, it } from "vitest";

describe("browser caption agreement", () => {
  it("keeps only complete words agreed across consecutive hypotheses", () => {
    expect(agree("we need a blue car", "we need a red car")).toEqual({
      text: "we need a red car",
      stable: "we need a",
    });
    expect(agree("the cat", "the cats").stable).toBe("the");
    expect(agree("", "hello").stable).toBe("");
  });
  it("commits on silence or bounds latency for uninterrupted speech", () => {
    expect(shouldCommit(599, 11999)).toBe(false);
    expect(shouldCommit(600, 2000)).toBe(true);
    expect(shouldCommit(0, 12000)).toBe(true);
  });
});
