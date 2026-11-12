import { describe, expect, it } from "vitest";
import { allowsEveryoneToDistribute } from "./distribution-policy";

describe("distribution policy", () => {
  it.each([
    [undefined, true],
    [[], true],
    [["*"], true],
    [["did:web:stream.place", "*"], true],
    [["did:web:stream.place"], false],
  ])("determines whether %j allows everyone", (broadcasters, expected) => {
    expect(allowsEveryoneToDistribute(broadcasters)).toBe(expected);
  });
});
