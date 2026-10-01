import { place } from "streamplace";
import { describe, expect, it } from "vitest";
import { parseTimedCaptions, timedCaptionsAt } from "./api";
import {
  activeLiveCaptions,
  displayLiveCaptions,
  LIVE_CAPTION_MAX_CUES_PER_TRACK,
  reduceLiveCaption,
  selectLiveCaptionTrack,
} from "./live-cues";
import { buildCaptionPolicy, readCaptionPolicy } from "./policy";
import { DEFAULT_CAPTION_PREFS, parseCaptionPrefs } from "./prefs";
import { mergeCaptionTracks, selectCaptionTrack } from "./tracks";

const en = { id: "a-en", language: "en", source: "auto", origin: "canonical" };
const es = { id: "h-es", language: "es", source: "human", origin: "record" };

function cueTime(ms: number): place.stream.caption.defs.LiveCue["startTime"] {
  return new Date(
    ms,
  ).toISOString() as place.stream.caption.defs.LiveCue["startTime"];
}

function liveCue(
  id: string,
  text: string,
  final: boolean,
  startTime = new Date(1000).toISOString(),
  track = en,
): place.stream.caption.defs.LiveCue {
  return {
    id,
    track,
    startTime: startTime as place.stream.caption.defs.LiveCue["startTime"],
    endTime: cueTime(Date.parse(startTime) + 20000),
    text,
    final,
  };
}

describe("reduceLiveCaption", () => {
  it("replaces an interim cue with a later revision", () => {
    let cues = reduceLiveCaption({}, liveCue("c", "hel", false), 1000);
    cues = reduceLiveCaption(cues, liveCue("c", "hello", false), 1100);
    expect(Object.values(cues)[0].text).toBe("hello");
    expect(Object.values(cues)[0].updatedAt).toBe(1100);
  });

  it("ignores late interim revisions after a final cue", () => {
    let cues = reduceLiveCaption({}, liveCue("c", "hello.", true), 1000);
    cues = reduceLiveCaption(cues, liveCue("c", "hello", false), 1100);
    expect(Object.values(cues)[0].text).toBe("hello.");
    expect(Object.values(cues)[0].final).toBe(true);
  });

  it("extends matching canonical finals without accepting text, start, or interim revisions", () => {
    const original = {
      ...liveCue("c", "Speech", true),
      endTime: cueTime(12000),
    };
    let cues = reduceLiveCaption({}, original, 1000);
    for (const revision of [
      { ...original, final: false },
      { ...original, text: "Changed" },
      { ...original, startTime: cueTime(2000) },
    ]) {
      cues = reduceLiveCaption(
        cues,
        { ...revision, endTime: cueTime(16000) },
        2000,
      );
      expect(Object.values(cues)[0].endMs).toBe(12000);
    }
    cues = reduceLiveCaption(
      cues,
      { ...original, endTime: cueTime(16000) },
      3000,
    );
    cues = reduceLiveCaption(cues, original, 4000);
    expect(
      activeLiveCaptions(cues, en.id, 14000).map((cue) => cue.text),
    ).toEqual(["Speech"]);
    expect(Object.values(cues)[0].updatedAt).toBe(3000);
    const sidecar = { ...original, track: { ...en, origin: "sidecar" } };
    let sidecars = reduceLiveCaption({}, sidecar, 1000);
    sidecars = reduceLiveCaption(
      sidecars,
      { ...sidecar, endTime: cueTime(16000) },
      3000,
    );
    expect(Object.values(sidecars)[0].endMs).toBe(12000);
  });

  it("prunes cues long past their last revision", () => {
    let cues = reduceLiveCaption({}, liveCue("old", "old", true), 0);
    cues = reduceLiveCaption(cues, liveCue("new", "new", false), 60000);
    expect(Object.values(cues).map((cue) => cue.id)).toEqual(["new"]);
  });

  it("bounds each track independently and keeps newest starts despite out-of-order arrivals and remote end times", () => {
    const count = LIVE_CAPTION_MAX_CUES_PER_TRACK + 64;
    let cues = reduceLiveCaption(
      {},
      liveCue("other", "Other track", true, undefined, es),
      10000,
    );
    for (let i = count - 1; i >= 0; i--) {
      cues = reduceLiveCaption(
        cues,
        {
          ...liveCue(
            `cue-${i}`,
            `Speech ${i}`,
            true,
            new Date(10000 + i).toISOString(),
          ),
          endTime: "2100-01-01T00:00:00.000Z",
        },
        10000,
      );
    }
    expect(
      Object.values(cues)
        .filter((cue) => cue.trackId === en.id)
        .map((cue) => cue.id)
        .sort(),
    ).toEqual(
      Array.from(
        { length: LIVE_CAPTION_MAX_CUES_PER_TRACK },
        (_, i) => `cue-${count - LIVE_CAPTION_MAX_CUES_PER_TRACK + i}`,
      ).sort(),
    );
    expect(
      Object.values(cues)
        .filter((cue) => cue.trackId === es.id)
        .map((cue) => cue.text),
    ).toEqual(["Other track"]);
    expect(
      activeLiveCaptions(cues, en.id, 11000).map((cue) => cue.text),
    ).toEqual([`Speech ${count - 2}`, `Speech ${count - 1}`]);
  });

  it("drops far-future cues against presentation rather than arrival time, including previously queued cues", () => {
    const atBoundary = liveCue(
      "boundary",
      "Queued speech",
      true,
      new Date(40000).toISOString(),
    );
    const cues = reduceLiveCaption({}, atBoundary, 1000, 10000);
    const next = reduceLiveCaption(
      cues,
      liveCue("too-far", "Never queue", true, new Date(40001).toISOString()),
      1000,
      10000,
    );
    expect(Object.values(next).map((cue) => cue.id)).toEqual(["boundary"]);
    expect(
      activeLiveCaptions(next, en.id, 40000).map((cue) => cue.text),
    ).toEqual(["Queued speech"]);
    const reanchored = reduceLiveCaption(
      next,
      liveCue("current", "Current speech", true, new Date(9000).toISOString()),
      1001,
      9000,
    );
    expect(Object.values(reanchored).map((cue) => cue.id)).toEqual(["current"]);
  });

  it("isolates revisions and finality for identical ids on different tracks", () => {
    let cues = reduceLiveCaption(
      {},
      liveCue("shared", "hello.", true, undefined, en),
      1000,
    );
    cues = reduceLiveCaption(
      cues,
      liveCue("shared", "hola", false, undefined, es),
      1100,
    );
    cues = reduceLiveCaption(
      cues,
      liveCue("shared", "late English", false, undefined, en),
      1200,
    );
    cues = reduceLiveCaption(
      cues,
      liveCue("shared", "hola.", true, undefined, es),
      1300,
    );
    cues = reduceLiveCaption(
      cues,
      liveCue("shared", "late Spanish", false, undefined, es),
      1400,
    );

    expect(
      activeLiveCaptions(cues, en.id, 1400).map((cue) => cue.text),
    ).toEqual(["hello."]);
    expect(
      activeLiveCaptions(cues, es.id, 1400).map((cue) => cue.text),
    ).toEqual(["hola."]);
    expect(Object.values(cues).every((cue) => cue.final)).toBe(true);
  });
});

describe("activeLiveCaptions", () => {
  it("shows live speech only within its media interval, not its arrival window", () => {
    const cues = reduceLiveCaption(
      {},
      {
        ...liveCue("cue", "Speech", true, new Date(10000).toISOString()),
        endTime: cueTime(30000),
      },
      50000,
    );
    expect(activeLiveCaptions(cues, en.id, 9000)).toEqual([]);
    expect(activeLiveCaptions(cues, en.id, 20000).map((c) => c.text)).toEqual([
      "Speech",
    ]);
    expect(activeLiveCaptions(cues, en.id, 30000)).toEqual([]);
  });
  it("shows node and pushed captions that arrive after their speech from arrival, for their duration", () => {
    const sidecar = { ...en, id: "s-en", origin: "sidecar" };
    const spoken = (text: string, final: boolean) => ({
      ...liveCue("s", text, final, cueTime(10000), sidecar),
      endTime: cueTime(12000),
    });
    // Recognition published speech from 10–12 s once the player was at 15 s.
    let cues = reduceLiveCaption({}, spoken("late words", false), 15000, 15000);
    expect(
      activeLiveCaptions(cues, sidecar.id, 15500).map((c) => c.text),
    ).toEqual(["late words"]);
    // The final revision stays where the interim appeared.
    cues = reduceLiveCaption(cues, spoken("late words.", true), 16000, 16000);
    expect(
      activeLiveCaptions(cues, sidecar.id, 16900).map((c) => c.text),
    ).toEqual(["late words."]);
    expect(activeLiveCaptions(cues, sidecar.id, 17100)).toEqual([]);
  });
  it("shows the newest two cues of the track, oldest first", () => {
    let cues = {};
    cues = reduceLiveCaption(
      cues,
      liveCue("1", "one", true, "2026-09-25T12:00:01.000Z"),
      1000,
      Date.parse("2026-09-25T12:00:04.000Z"),
    );
    cues = reduceLiveCaption(
      cues,
      liveCue("3", "three", false, "2026-09-25T12:00:03.000Z"),
      1000,
      Date.parse("2026-09-25T12:00:04.000Z"),
    );
    cues = reduceLiveCaption(
      cues,
      liveCue("2", "two", true, "2026-09-25T12:00:02.000Z"),
      1000,
      Date.parse("2026-09-25T12:00:04.000Z"),
    );
    cues = reduceLiveCaption(
      cues,
      liveCue("x", "otra", true, "2026-09-25T12:00:04.000Z", es),
      1000,
      Date.parse("2026-09-25T12:00:04.000Z"),
    );
    expect(
      activeLiveCaptions(
        cues,
        en.id,
        Date.parse("2026-09-25T12:00:04.000Z"),
      ).map((c) => c.text),
    ).toEqual(["two", "three"]);
  });
});

describe("displayLiveCaptions", () => {
  it("keeps delayed OBS speech readable on arrival and replaces it with newer speech", () => {
    let cues = reduceLiveCaption(
      {},
      {
        ...liveCue("cue", "Speech", true, new Date(10000).toISOString()),
        endTime: cueTime(30000),
      },
      50000,
    );
    expect(displayLiveCaptions(cues, en.id, 56000).map((c) => c.text)).toEqual([
      "Speech",
    ]);
    expect(displayLiveCaptions(cues, en.id, 70000)).toEqual([]);
    cues = reduceLiveCaption(
      cues,
      {
        ...liveCue("next", "Next speech", true, new Date(31000).toISOString()),
        endTime: cueTime(32000),
      },
      57000,
    );
    expect(displayLiveCaptions(cues, en.id, 58000).map((c) => c.text)).toEqual([
      "Next speech",
    ]);
    expect(displayLiveCaptions(cues, en.id, 62000)).toEqual([]);
  });
});

describe("live caption track selection", () => {
  it.each([activeLiveCaptions, displayLiveCaptions])(
    "follows canonical takeover and falls back to sidecars, unless explicitly selected (%s)",
    (display) => {
      const human = { ...en, id: "human-en", source: "human" };
      const sidecar = { ...en, id: "node-en", origin: "sidecar" };
      const options = [en, human, sidecar];
      let cues = reduceLiveCaption(
        {},
        {
          ...liveCue("auto", "Old automatic", true),
          endTime: cueTime(2000),
        },
        1000,
      );
      cues = reduceLiveCaption(
        cues,
        liveCue(
          "human",
          "Human speech",
          true,
          new Date(7000).toISOString(),
          human,
        ),
        7000,
      );
      cues = reduceLiveCaption(
        cues,
        liveCue(
          "node",
          "Node speech",
          true,
          new Date(8000).toISOString(),
          sidecar,
        ),
        8000,
      );
      expect(
        selectLiveCaptionTrack(options, null, cues, 9000, display)?.id,
      ).toBe(human.id);
      expect(
        selectLiveCaptionTrack(options, en.id, cues, 9000, display)?.id,
      ).toBe(en.id);
      expect(
        selectLiveCaptionTrack(options, null, cues, 27500, display)?.id,
      ).toBe(sidecar.id);
    },
  );
});

describe("mergeCaptionTracks", () => {
  it("attaches element tracks to server tracks by language and label", () => {
    const options = mergeCaptionTracks(
      [en, { ...en, id: "b-en", label: "English (live)" }, en],
      [
        { key: "0", language: "en", label: "English (live)" },
        { key: "1", language: "fr", label: "Français" },
      ],
    );
    expect(options.map((o) => [o.id, o.elementKey])).toEqual([
      ["a-en", undefined],
      ["b-en", "0"],
      ["element:1", "1"],
    ]);
  });

  it("normalizes exact locale tags without merging distinct regions", () => {
    const options = mergeCaptionTracks(
      [
        {
          id: "pt-br",
          language: "pt-BR",
          source: "auto",
          origin: "canonical",
        },
      ],
      [
        { key: "pt", language: "pt-PT", label: "Português (Portugal)" },
        { key: "br", language: "pt_BR", label: "Português (Brasil)" },
      ],
    );

    expect(options.map((option) => [option.id, option.elementKey])).toEqual([
      ["pt-br", "br"],
      ["element:pt", "pt"],
    ]);
  });
});

describe("selectCaptionTrack", () => {
  const options = mergeCaptionTracks(
    [en, es, { ...es, id: "mx", language: "es-MX" }],
    [],
  );

  it("keeps an explicit pick while it is offered", () => {
    expect(selectCaptionTrack(options, "h-es", "en")?.id).toBe("h-es");
    expect(selectCaptionTrack(options, "gone", "es-MX")?.id).toBe("mx");
  });

  it("normalizes locale tags before exact preference matching", () => {
    const regional = mergeCaptionTracks(
      [
        { ...es, id: "pt-pt", language: "pt-PT" },
        { ...es, id: "pt-br", language: "pt-BR" },
      ],
      [],
    );
    expect(selectCaptionTrack(regional, null, "PT_br")?.id).toBe("pt-br");
  });

  it("falls back to the primary language, then the first track", () => {
    expect(selectCaptionTrack(options, null, "es-AR")?.id).toBe("h-es");
    expect(selectCaptionTrack(options, null, "de")?.id).toBe("a-en");
    expect(selectCaptionTrack([], null, "en")).toBeNull();
  });
});

describe("timedCaptionsAt", () => {
  const cues = parseTimedCaptions({
    cues: [
      { startMs: 2000, endMs: 3000, text: "b" },
      { startMs: 0, endMs: 1000, text: "a" },
      { startMs: 2500, endMs: 4000, text: "c" },
      { startMs: "bad", endMs: 1, text: "x" },
    ],
  });

  it("returns the cues covering a position", () => {
    expect(timedCaptionsAt(cues, 500).map((c) => c.text)).toEqual(["a"]);
    expect(timedCaptionsAt(cues, 1000)).toEqual([]);
    expect(timedCaptionsAt(cues, 2700).map((c) => c.text)).toEqual(["b", "c"]);
    expect(timedCaptionsAt(cues, 3500).map((c) => c.text)).toEqual(["c"]);
  });

  it("finds a long-running VOD cue behind many expired overlapping cues", () => {
    const overlapping = [
      { startMs: 0, endMs: 60000, text: "Long speech" },
      ...Array.from({ length: 12 }, (_, i) => ({
        startMs: (i + 1) * 1000,
        endMs: (i + 1) * 1000 + 500,
        text: "Short",
      })),
    ];
    expect(timedCaptionsAt(overlapping, 15000).map((c) => c.text)).toEqual([
      "Long speech",
    ]);
  });
});

describe("parseCaptionPrefs", () => {
  it("keeps valid fields and defaults the rest", () => {
    const prefs = parseCaptionPrefs(
      JSON.stringify({ size: 150, textColor: "yellow", edge: "glow" }),
    );
    expect(prefs).toEqual({
      ...DEFAULT_CAPTION_PREFS,
      size: 150,
      textColor: "yellow",
    });
    expect(parseCaptionPrefs("{not json")).toEqual(DEFAULT_CAPTION_PREFS);
  });
});

describe("caption policy", () => {
  it("treats an absent policy as automatic captions with node captions allowed", () => {
    expect(readCaptionPolicy(undefined)).toEqual({
      mode: "auto",
      allowNodeCaptions: true,
      languages: [],
    });
  });

  it("round-trips the dashboard settings", () => {
    const settings = {
      mode: "ingest" as const,
      allowNodeCaptions: false,
      languages: ["en", "es"],
    };
    expect(readCaptionPolicy(buildCaptionPolicy(settings))).toEqual(settings);
  });
});
