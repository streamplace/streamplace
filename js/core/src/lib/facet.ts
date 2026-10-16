// Taken from ATcute's richtext-segmenter
// https://github.com/mary-ext/atcute/blob/trunk/packages/bluesky/richtext-segmenter/lib/index.ts
// repoed b/c we need to import types from @atproto/api not @atcute/bsky
import { RichText } from "@atproto/api";
import { Main } from "@atproto/api/dist/client/types/app/bsky/richtext/facet";
import type { place } from "streamplace";

type UnwrapArray<T> = T extends (infer V)[] ? V : never;

export type Facet = Main;
export type FacetFeature = UnwrapArray<Facet["features"]>;

export interface RichtextSegment {
  text: string;
  features: FacetFeature[] | undefined;
}

const segment = (
  text: string,
  features: FacetFeature[] | undefined,
): RichtextSegment => {
  return { text, features: text.length > 0 ? features : undefined };
};

export const segmentize = (
  text: string,
  facets: Facet[] | undefined,
): RichtextSegment[] => {
  if (facets === undefined || facets.length === 0) {
    return [segment(text, undefined)];
  }

  const segments: RichtextSegment[] = [];
  const utf16Length = text.length;
  let utf16Cursor = 0;
  let utf8Cursor = 0;

  const advanceCursor = (startUtf16: number, endUtf8: number): number => {
    let curs = startUtf16;

    // Fast-path for entirely ASCII text
    const isLikelyAsciiText = text.charCodeAt(curs) < 0x80;
    if (isLikelyAsciiText) {
      curs += 1;
      utf8Cursor += 1;

      // SIMD-like batch processing
      while (utf8Cursor + 8 <= endUtf8 && curs + 8 <= utf16Length) {
        const char1 = text.charCodeAt(curs);
        const char2 = text.charCodeAt(curs + 1);
        const char3 = text.charCodeAt(curs + 2);
        const char4 = text.charCodeAt(curs + 3);
        const char5 = text.charCodeAt(curs + 4);
        const char6 = text.charCodeAt(curs + 5);
        const char7 = text.charCodeAt(curs + 6);
        const char8 = text.charCodeAt(curs + 7);

        if (
          (char1 | char2 | char3 | char4 | char5 | char6 | char7 | char8) <
          0x80
        ) {
          curs += 8;
          utf8Cursor += 8;
          continue;
        }

        break;
      }
    }

    while (utf8Cursor < endUtf8 && curs < utf16Length) {
      const code = text.charCodeAt(curs);

      if (code < 0x80) {
        curs += 1;
        utf8Cursor += 1;
      } else if (code < 0x800) {
        curs += 1;
        utf8Cursor += 2;
      } else if (code < 0xd800 || code > 0xdbff) {
        curs += 1;
        utf8Cursor += 3;
      } else {
        curs += 2;
        utf8Cursor += 4;
      }
    }

    return curs;
  };

  for (let idx = 0, len = facets.length; idx < len; idx++) {
    const facet = facets[idx];

    const { byteStart, byteEnd } = facet.index;
    const features = facet.features;

    if (byteStart > byteEnd || !features || features.length === 0) {
      continue;
    }

    if (utf8Cursor < byteStart) {
      const nextUtf16Cursor = advanceCursor(utf16Cursor, byteStart);
      if (nextUtf16Cursor > utf16Cursor) {
        segments.push(
          segment(text.slice(utf16Cursor, nextUtf16Cursor), undefined),
        );
      }

      utf16Cursor = nextUtf16Cursor;
    }

    {
      const nextUtf16Cursor = advanceCursor(utf16Cursor, byteEnd);
      if (nextUtf16Cursor > utf16Cursor) {
        segments.push(
          segment(text.slice(utf16Cursor, nextUtf16Cursor), features),
        );
      }

      utf16Cursor = nextUtf16Cursor;
    }
  }

  if (utf16Cursor < utf16Length) {
    segments.push(segment(text.slice(utf16Cursor), undefined));
  }

  return segments;
};

/** The facet-level $type the place.stream.richtext.facet lexicon requires. */
const CHAT_FACET_TYPE = "place.stream.richtext.facet";

/** A chat facet: the place.stream.richtext.facet the chat lexicons require. */
export type ChatFacet = place.stream.richtext.facet.Main;

/** Feature types its features union allows: the app.bsky link and mention defs. */
const CHAT_FACET_FEATURE_TYPES: Record<string, true> = {
  "app.bsky.richtext.facet#link": true,
  "app.bsky.richtext.facet#mention": true,
};

/**
 * The facets for a chat message body: links and mentions, each stamped with
 * the facet-level $type the chat lexicon requires.
 *
 * detectFacets types facets as app.bsky's (on mentions) or leaves the field
 * off entirely (on links, and so on any facet it builds itself). A record
 * carrying either fails validation against place.stream.richtext.facet, which
 * requires that literal: "Expected place.stream.richtext.facet (got
 * app.bsky.richtext.facet)". Normalize here, where chat facets are made.
 */
export const detectChatFacets = async (
  agent: Parameters<RichText["detectFacets"]>[0],
  text: string,
): Promise<ChatFacet[] | undefined> => {
  const rt = new RichText({ text });
  await rt.detectFacets(agent);
  const facets = rt.facets
    ?.filter((facet) =>
      facet.features.every(
        (feature) => CHAT_FACET_FEATURE_TYPES[feature.$type] === true,
      ),
    )
    .map(
      // detectFacets hands back plain strings where the generated facet types
      // want did:/scheme: templates; the lexicon validator checks those, not tsc.
      (facet) => ({ ...facet, $type: CHAT_FACET_TYPE }) as ChatFacet,
    );
  return facets?.length ? facets : undefined;
};
