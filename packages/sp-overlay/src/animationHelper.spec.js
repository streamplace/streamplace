const propsify = (...things) => {
  return things.map(thing => {
    thing;
  });
};

describe("getTweens", () => {
  it("should calculate tweens", () => {
    const input = propsify([
      { time: 0, left: 0, top: 0 },
      { time: 1000, left: 100 },
      { time: 2000, left: 0, top: 100 },
      { time: 3000, left: 100 },
      { time: 4000, top: 0, left: 0 }
    ]);

    const output = [
      { prop: "left", startTime: 0, endTime: 1000, startVal: 0, endVal: 100 },
      {
        prop: "left",
        startTime: 1000,
        endTime: 2000,
        startVal: 100,
        endVal: 0
      },
      {
        prop: "left",
        startTime: 2000,
        endTime: 3000,
        startVal: 0,
        endVal: 100
      },
      {
        prop: "left",
        startTime: 3000,
        endTime: 4000,
        startVal: 100,
        endVal: 0
      },
      { prop: "top", startTime: 0, endTime: 2000, startVal: 0, endVal: 100 },
      { prop: "top", startTime: 2000, endTime: 4000, startVal: 100, endVal: 0 }
    ];
  });
});
