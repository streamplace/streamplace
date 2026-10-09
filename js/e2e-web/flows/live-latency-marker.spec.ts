import { expect, test } from "@playwright/test";
import { makeLatencyMarker } from "./live-latency-marker";

// Pure marker-corruption and boundary coverage for the latency measurement:
// these need no page, only the suite's normal harness workflow.
const marker = makeLatencyMarker();
const brightness = (bits: number[]): number[] =>
  bits.map((bit) => (bit ? 232 : 24));

test("frame IDs survive bounded compression noise, including unsigned boundaries", () => {
  for (const id of [0, 1, 65535, 65536, 0x80000000, 0xffffffff]) {
    expect(marker.decode(brightness(marker.encode(id)))).toBe(id);
  }
});

test("a damaged header, frame ID, or checksum cannot become a latency sample", () => {
  const pixels = brightness(marker.encode(0x12345678));
  for (let bit = 0; bit < pixels.length; bit++) {
    const damaged = [...pixels];
    damaged[bit] = 255 - damaged[bit];
    expect(marker.decode(damaged), `damaged bit ${bit}`).toBe(null);
  }
});

test("uncertain or incomplete markers are rejected", () => {
  const pixels = brightness(marker.encode(42));
  expect(marker.decode(pixels.slice(1))).toBe(null);
  for (const value of [128, -1, 256, Number.NaN, Number.POSITIVE_INFINITY]) {
    const damaged = [...pixels];
    damaged[20] = value;
    expect(marker.decode(damaged)).toBe(null);
  }
});

test("invalid frame IDs fail before producing pixels", () => {
  for (const id of [
    -1,
    0x100000000,
    1.5,
    Number.NaN,
    Number.POSITIVE_INFINITY,
  ]) {
    expect(() => marker.encode(id)).toThrow(RangeError);
  }
});
