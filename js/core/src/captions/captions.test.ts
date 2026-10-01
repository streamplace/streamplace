import { place } from "streamplace";
import { describe, expect, it } from "vitest";
import { parseTimedCaptions, timedCaptionsAt } from "./api";
import { activeLiveCaptions, reduceLiveCaption } from "./live-cues";
import { buildCaptionPolicy, readCaptionPolicy } from "./policy";
import { DEFAULT_CAPTION_PREFS, parseCaptionPrefs } from "./prefs";
import { mergeCaptionTracks, selectCaptionTrack } from "./tracks";

const en = { id: "a-en", language: "en", source: "auto", origin: "canonical" };
const es = { id: "h-es", language: "es", source: "human", origin: "record" };

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
    endTime: new Date(Date.parse(startTime) + 20000).toISOString(),
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

  it("never revises a final cue", () => {
    let cues = reduceLiveCaption({}, liveCue("c", "hello.", true), 1000);
    cues = reduceLiveCaption(cues, liveCue("c", "hello", false), 1100);
    expect(Object.values(cues)[0].text).toBe("hello.");
    expect(Object.values(cues)[0].final).toBe(true);
  });

  it("prunes cues long past their last revision", () => {
    let cues = reduceLiveCaption({}, liveCue("old", "old", true), 0);
    cues = reduceLiveCaption(cues, liveCue("new", "new", false), 60000);
    expect(Object.values(cues).map((cue) => cue.id)).toEqual(["new"]);
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
  it("shows the newest two cues of the track, oldest first", () => {
    let cues = {};
    cues = reduceLiveCaption(
      cues,
      liveCue("1", "one", true, "2026-09-25T12:00:01.000Z"),
      1000,
    );
    cues = reduceLiveCaption(
      cues,
      liveCue("3", "three", false, "2026-09-25T12:00:03.000Z"),
      1000,
    );
    cues = reduceLiveCaption(
      cues,
      liveCue("2", "two", true, "2026-09-25T12:00:02.000Z"),
      1000,
    );
    cues = reduceLiveCaption(
      cues,
      liveCue("x", "otra", true, "2026-09-25T12:00:04.000Z", es),
      1000,
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
