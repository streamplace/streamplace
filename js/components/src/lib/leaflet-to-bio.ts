import { BlobRef } from "@atproto/api";
import {
  PlaceStreamBioBlocksBlockquote,
  PlaceStreamBioBlocksBskyPost,
  PlaceStreamBioBlocksDivider,
  PlaceStreamBioBlocksEmbed,
  PlaceStreamBioBlocksHeader,
  PlaceStreamBioBlocksImage,
  PlaceStreamBioBlocksLink,
  PlaceStreamBioBlocksOrderedList,
  PlaceStreamBioBlocksText,
  PlaceStreamBioBlocksUnorderedList,
  PlaceStreamBioLayoutsPanels,
  PlaceStreamBioPage,
  PlaceStreamBioRichtextFacet,
} from "streamplace";

// Minimal subset of pub.leaflet.* shapes that we know how to translate. Fields
// we don't read are intentionally absent so unknown leaflet block variants fall
// through to the catch-all branch and produce a warning.
//
// Keep these structural — they're parsed from JSON returned by the leaflet
// author's PDS, so we never trust the values blindly.

interface LeafletAspectRatio {
  width: number;
  height: number;
}

interface LeafletContent {
  $type: "pub.leaflet.content";
  pages?: LeafletPage[];
}

interface LeafletDoc {
  $type?: string;
  title?: string;
  description?: string;
  // pub.leaflet.document (old)
  pages?: LeafletPage[];
  // site.standard.document (new)
  content?: LeafletContent;
}

type LeafletPage = LeafletLinearPage | LeafletCanvasPage | { $type?: string };

interface LeafletLinearPage {
  $type: "pub.leaflet.pages.linearDocument";
  blocks?: LeafletBlockEntry[];
}

interface LeafletCanvasPage {
  $type: "pub.leaflet.pages.canvas";
  blocks?: LeafletBlockEntry[];
}

interface LeafletBlockEntry {
  block?: LeafletBlock;
  alignment?: string;
}

type LeafletBlock = { $type: string; [k: string]: unknown };

export interface LeafletFlatBlock {
  idx: number;
  pageIdx: number;
  label: string;
  blockType: string;
  // Passed through to leafletRangesToBio — do not access from UI code
  _entry: unknown;
}

export interface PanelRange {
  startIdx: number;
  endIdx: number; // inclusive
  color: string;
}

const PANEL_RANGE_COLORS = [
  "#e74c3c",
  "#3498db",
  "#2ecc71",
  "#f39c12",
  "#9b59b6",
  "#1abc9c",
  "#e91e63",
  "#f1c40f",
];

export function autoSplitRanges(
  blocks: LeafletFlatBlock[],
  colors: string[] = PANEL_RANGE_COLORS,
): PanelRange[] {
  const ranges: PanelRange[] = [];
  let runStart: number | null = null;

  for (let i = 0; i < blocks.length; i++) {
    const isDivider =
      blocks[i].blockType === "divider" ||
      blocks[i].blockType === "horizontalRule";

    if (isDivider) {
      if (runStart !== null) {
        ranges.push({
          startIdx: runStart,
          endIdx: i - 1,
          color: colors[ranges.length % colors.length],
        });
        runStart = null;
      }
      continue;
    }

    if (runStart === null) {
      runStart = i;
    }
  }

  if (runStart !== null) {
    ranges.push({
      startIdx: runStart,
      endIdx: blocks.length - 1,
      color: colors[ranges.length % colors.length],
    });
  }

  return ranges;
}

export function extractLeafletBlocks(doc: LeafletDoc): {
  blocks: LeafletFlatBlock[];
  warnings: string[];
} {
  const warnings: string[] = [];
  const blocks: LeafletFlatBlock[] = [];

  if (
    doc.content !== undefined &&
    doc.content?.$type !== "pub.leaflet.content"
  ) {
    throw new Error(
      `This record is not from leaflet.pub (content type: ${doc.content?.$type ?? "unknown"}).`,
    );
  }

  const pages = Array.isArray(doc.content?.pages)
    ? doc.content.pages
    : Array.isArray(doc.pages)
      ? doc.pages
      : [];
  const linearOrCanvasPages = pages.filter(
    (p): p is LeafletLinearPage | LeafletCanvasPage =>
      p?.$type === "pub.leaflet.pages.linearDocument" ||
      p?.$type === "pub.leaflet.pages.canvas",
  );
  const skippedPages = pages.length - linearOrCanvasPages.length;
  if (skippedPages > 0) {
    warnings.push(
      `Skipped ${skippedPages} page(s) of unknown type. Only linearDocument and canvas pages are supported.`,
    );
  }

  linearOrCanvasPages.forEach((page, pageIdx) => {
    for (const entry of page.blocks ?? []) {
      if (!entry.block) continue;
      const idx = blocks.length;
      const blockType = entry.block.$type?.split(".").pop() ?? "unknown";
      const label = labelForBlock(entry.block);
      blocks.push({ idx, pageIdx, label, blockType, _entry: entry });
    }
  });

  return { blocks, warnings };
}

export function leafletRangesToBio(
  doc: LeafletDoc,
  blocks: LeafletFlatBlock[],
  ranges: PanelRange[],
  opts: LeafletToBioOptions = {},
): LeafletToBioResult {
  const warnings: string[] = [];
  const sortedRanges = [...ranges].sort((a, b) => a.startIdx - b.startIdx);

  const panels: PlaceStreamBioLayoutsPanels.Panel[] = [];
  for (const range of sortedRanges) {
    const panelBlocks = blocks
      .slice(range.startIdx, range.endIdx + 1)
      .map((fb) =>
        translateBlockEntry(fb._entry as LeafletBlockEntry, warnings),
      )
      .filter((e): e is PlaceStreamBioLayoutsPanels.BlockEntry => e !== null);
    if (panelBlocks.length > 0) {
      panels.push({ blocks: panelBlocks });
    }
  }

  const selectedCount = sortedRanges.reduce(
    (sum, r) => sum + (r.endIdx - r.startIdx + 1),
    0,
  );
  const droppedCount = blocks.length - selectedCount;
  if (droppedCount > 0) {
    warnings.push(
      `${droppedCount} block(s) were not included in any panel and were dropped.`,
    );
  }

  const layout: PlaceStreamBioLayoutsPanels.Main = {
    $type: "place.stream.bio.layouts.panels",
    panels: panels.length > 0 ? panels : [{ blocks: [] }],
  };

  const now = opts.now ?? new Date().toISOString();
  const prior = opts.preserve ?? null;

  const bio: PlaceStreamBioPage.Record = {
    $type: "place.stream.bio.page",
    layout: layout as PlaceStreamBioPage.Record["layout"],
    createdAt: prior?.createdAt ?? now,
    updatedAt: now,
  };

  if (typeof doc.description === "string" && doc.description.length > 0) {
    bio.description = { plaintext: doc.description };
  }
  if (prior?.socials && prior.socials.length > 0) {
    bio.socials = prior.socials;
  }
  if (opts.importedFrom) {
    bio.importedFrom = opts.importedFrom;
  }

  return { bio, warnings };
}

function labelForBlock(block: LeafletBlock): string {
  switch (block.$type) {
    case "pub.leaflet.blocks.text":
    case "pub.leaflet.blocks.header":
    case "pub.leaflet.blocks.blockquote":
      return truncateStr(String(block.plaintext ?? ""), 60) || block.$type;
    case "pub.leaflet.blocks.image":
      return block.alt
        ? `Image: ${truncateStr(String(block.alt), 50)}`
        : "Image";
    case "pub.leaflet.blocks.button":
      return `Link: ${truncateStr(String(block.text || block.url || ""), 50)}`;
    case "pub.leaflet.blocks.website":
      return `Embed: ${truncateStr(String(block.title || block.src || ""), 50)}`;
    case "pub.leaflet.blocks.iframe":
      return `Embed: ${truncateStr(String(block.src ?? ""), 50)}`;
    case "pub.leaflet.blocks.bskyPost":
      return "Bluesky Post";
    case "pub.leaflet.blocks.horizontalRule":
      return "Divider";
    case "pub.leaflet.blocks.orderedList":
      return "Ordered List";
    case "pub.leaflet.blocks.unorderedList":
      return "Unordered List";
    default:
      return block.$type?.split(".").pop() ?? "Block";
  }
}

function truncateStr(s: string, max: number): string {
  return s.length <= max ? s : s.slice(0, max - 1) + "…";
}

export interface LeafletToBioOptions {
  /** AT-URI of the leaflet record being imported, recorded on the bio for re-import UX. */
  importedFrom?: string;
  /** Existing bio whose streamplace-only fields (socials, livestream/schedule/socialLinks blocks) should be preserved. */
  preserve?: PlaceStreamBioPage.Record | null;
  /** ISO timestamp to record on the new bio. Defaults to now. */
  now?: string;
}

export interface LeafletToBioResult {
  bio: PlaceStreamBioPage.Record;
  /** Human-readable notes about what was skipped or remapped. Surface to the user after import. */
  warnings: string[];
}

/**
 * Translate a pub.leaflet.document record into a place.stream.bio record.
 *
 * Mapping:
 * - doc.description           → bio.description (no facets — leaflet's description is plain string)
 * - linearDocument pages      → flattened into one column, joined by divider blocks between pages
 * - canvas pages              → blocks read in array order (positions discarded), divider between pages
 * - leaflet text/header/...   → identical streamplace block, $type rewritten
 * - leaflet button            → bio link block
 * - leaflet website / iframe  → bio embed block
 * - leaflet horizontalRule    → bio divider
 * - leaflet code/math/poll/page, unknown blocks → skipped + warning
 *
 * Streamplace-only fields on `preserve` (socials, and any livestream/schedule/socialLinks
 * blocks in the prior layout) are NOT copied over — the body is replaced wholesale.
 * The caller can do a smarter merge if desired; we keep the translator pure.
 */
export function leafletDocToBio(
  doc: LeafletDoc,
  opts: LeafletToBioOptions = {},
): LeafletToBioResult {
  const warnings: string[] = [];
  const blocks: PlaceStreamBioLayoutsPanels.BlockEntry[] = [];

  if (
    doc.content !== undefined &&
    doc.content?.$type !== "pub.leaflet.content"
  ) {
    throw new Error(
      `This record is not from leaflet.pub (content type: ${doc.content?.$type ?? "unknown"}).`,
    );
  }

  const pages = Array.isArray(doc.content?.pages)
    ? doc.content.pages
    : Array.isArray(doc.pages)
      ? doc.pages
      : [];
  const linearOrCanvasPages = pages.filter(
    (p): p is LeafletLinearPage | LeafletCanvasPage =>
      p?.$type === "pub.leaflet.pages.linearDocument" ||
      p?.$type === "pub.leaflet.pages.canvas",
  );
  const skippedPages = pages.length - linearOrCanvasPages.length;
  if (skippedPages > 0) {
    warnings.push(
      `Skipped ${skippedPages} page(s) of unknown type. Only linearDocument and canvas pages are supported.`,
    );
  }

  linearOrCanvasPages.forEach((page, pageIndex) => {
    if (pageIndex > 0) {
      blocks.push(makeDividerEntry());
    }
    if (page.$type === "pub.leaflet.pages.canvas") {
      warnings.push(
        `Imported page ${pageIndex + 1} as a flat list — canvas positioning is not preserved on bios.`,
      );
    }
    for (const entry of page.blocks ?? []) {
      const translated = translateBlockEntry(entry, warnings);
      if (translated) blocks.push(translated);
    }
  });

  const layout: PlaceStreamBioLayoutsPanels.Main = {
    $type: "place.stream.bio.layouts.panels",
    panels: [{ blocks }],
  };

  const now = opts.now ?? new Date().toISOString();
  const prior = opts.preserve ?? null;

  const bio: PlaceStreamBioPage.Record = {
    $type: "place.stream.bio.page",
    layout: layout as PlaceStreamBioPage.Record["layout"],
    createdAt: prior?.createdAt ?? now,
    updatedAt: now,
  };

  if (typeof doc.description === "string" && doc.description.length > 0) {
    bio.description = { plaintext: doc.description };
  }

  if (prior?.socials && prior.socials.length > 0) {
    bio.socials = prior.socials;
  }

  if (opts.importedFrom) {
    bio.importedFrom = opts.importedFrom;
  }

  return { bio, warnings };
}

function translateBlockEntry(
  entry: LeafletBlockEntry,
  warnings: string[],
): PlaceStreamBioLayoutsPanels.BlockEntry | null {
  const block = entry?.block;
  if (!block || typeof block.$type !== "string") return null;

  const translated = translateBlock(block, warnings);
  if (!translated) return null;

  const result: PlaceStreamBioLayoutsPanels.BlockEntry = {
    $type: "place.stream.bio.layouts.panels#blockEntry",
    block: translated as PlaceStreamBioLayoutsPanels.BlockEntry["block"],
  };

  const alignment = entry.alignment;
  if (alignment === "left" || alignment === "center" || alignment === "right") {
    result.alignment = alignment;
  }
  // Leaflet supports "justify"; we don't, so we drop it silently. Users are unlikely
  // to notice and it would just produce noise in the warnings panel.

  return result;
}

type StreamplaceBlock = any;

function translateBlock(
  block: LeafletBlock,
  warnings: string[],
): StreamplaceBlock | null {
  switch (block.$type) {
    case "pub.leaflet.blocks.text":
      return translateText(block);
    case "pub.leaflet.blocks.header":
      return translateHeader(block);
    case "pub.leaflet.blocks.image":
      return translateImage(block);
    case "pub.leaflet.blocks.blockquote":
      return translateBlockquote(block);
    case "pub.leaflet.blocks.horizontalRule":
      return makeDivider();
    case "pub.leaflet.blocks.button":
      return translateButton(block);
    case "pub.leaflet.blocks.website":
      return translateWebsite(block);
    case "pub.leaflet.blocks.iframe":
      return translateIframe(block);
    case "pub.leaflet.blocks.bskyPost":
      return translateBskyPost(block);
    case "pub.leaflet.blocks.unorderedList":
      return translateUnorderedList(block, warnings);
    case "pub.leaflet.blocks.orderedList":
      return translateOrderedList(block, warnings);
    case "pub.leaflet.blocks.code":
    case "pub.leaflet.blocks.math":
    case "pub.leaflet.blocks.poll":
    case "pub.leaflet.blocks.page":
      warnings.push(`Skipped unsupported block: ${block.$type}.`);
      return null;
    default:
      warnings.push(`Skipped unknown block: ${block.$type}.`);
      return null;
  }
}

function translateText(block: LeafletBlock): PlaceStreamBioBlocksText.Main {
  const out: PlaceStreamBioBlocksText.Main = {
    $type: "place.stream.bio.blocks.text",
    plaintext: stringOr(block.plaintext, ""),
  };
  const facets = passthroughFacets(block.facets);
  if (facets) out.facets = facets;
  const textSize = block.textSize;
  if (textSize === "default" || textSize === "small" || textSize === "large") {
    out.textSize = textSize;
  }
  return out;
}

function translateHeader(block: LeafletBlock): PlaceStreamBioBlocksHeader.Main {
  const out: PlaceStreamBioBlocksHeader.Main = {
    $type: "place.stream.bio.blocks.header",
    plaintext: stringOr(block.plaintext, ""),
  };
  const facets = passthroughFacets(block.facets);
  if (facets) out.facets = facets;
  // Leaflet supports h1-h6; bios cap at h3. Levels 4-6 collapse to h3.
  const level = typeof block.level === "number" ? block.level : undefined;
  if (level) out.level = Math.max(1, Math.min(3, Math.round(level)));
  return out;
}

function translateBlockquote(
  block: LeafletBlock,
): PlaceStreamBioBlocksBlockquote.Main {
  const out: PlaceStreamBioBlocksBlockquote.Main = {
    $type: "place.stream.bio.blocks.blockquote",
    plaintext: stringOr(block.plaintext, ""),
  };
  const facets = passthroughFacets(block.facets);
  if (facets) out.facets = facets;
  return out;
}

function translateImage(
  block: LeafletBlock,
): PlaceStreamBioBlocksImage.Main | null {
  // The blob ref must be re-referenced under the user's PDS for the new record.
  // When importing from the user's own leaflet doc this is a no-op (same PDS,
  // same CID); cross-PDS imports require the caller to re-upload first.
  const image = block.image;
  if (!(image instanceof BlobRef)) return null;
  const ar = block.aspectRatio as LeafletAspectRatio | undefined;
  if (!ar || typeof ar.width !== "number" || typeof ar.height !== "number") {
    return null;
  }
  const out: PlaceStreamBioBlocksImage.Main = {
    $type: "place.stream.bio.blocks.image",
    image,
    aspectRatio: { width: ar.width, height: ar.height },
  };
  if (typeof block.alt === "string") out.alt = block.alt;
  return out;
}

function translateButton(block: LeafletBlock): PlaceStreamBioBlocksLink.Main {
  return {
    $type: "place.stream.bio.blocks.link",
    url: stringOr(block.url, ""),
    text: stringOr(block.text, ""),
  };
}

function translateWebsite(block: LeafletBlock): PlaceStreamBioBlocksEmbed.Main {
  const out: PlaceStreamBioBlocksEmbed.Main = {
    $type: "place.stream.bio.blocks.embed",
    src: stringOr(block.src, ""),
  };
  if (typeof block.title === "string") out.title = block.title;
  if (typeof block.description === "string")
    out.description = block.description;
  if (block.previewImage instanceof BlobRef)
    out.previewImage = block.previewImage;
  return out;
}

function translateIframe(block: LeafletBlock): PlaceStreamBioBlocksEmbed.Main {
  return {
    $type: "place.stream.bio.blocks.embed",
    src: stringOr(block.src, ""),
  };
}

function translateBskyPost(
  block: LeafletBlock,
): PlaceStreamBioBlocksBskyPost.Main | null {
  const post = block.post as { uri?: string; cid?: string } | undefined;
  if (!post || typeof post.uri !== "string" || typeof post.cid !== "string") {
    return null;
  }
  return {
    $type: "place.stream.bio.blocks.bskyPost",
    post: { uri: post.uri, cid: post.cid },
  };
}

function translateOrderedList(
  block: LeafletBlock,
  warnings: string[],
): PlaceStreamBioBlocksOrderedList.Main {
  const children = Array.isArray(block.children) ? block.children : [];
  return {
    $type: "place.stream.bio.blocks.orderedList",
    children: children
      .map((c) => translateOrderedListItem(c, warnings))
      .filter((x): x is PlaceStreamBioBlocksOrderedList.ListItem => x !== null),
  };
}

function translateUnorderedList(
  block: LeafletBlock,
  warnings: string[],
): PlaceStreamBioBlocksUnorderedList.Main {
  const children = Array.isArray(block.children) ? block.children : [];
  return {
    $type: "place.stream.bio.blocks.unorderedList",
    children: children
      .map((c) => translateUnorderedListItem(c, warnings))
      .filter(
        (x): x is PlaceStreamBioBlocksUnorderedList.ListItem => x !== null,
      ),
  };
}

function translateOrderedListItem(
  raw: unknown,
  warnings: string[],
): PlaceStreamBioBlocksOrderedList.ListItem | null {
  const item = raw as
    | {
        content?: LeafletBlock;
        children?: unknown[];
        unorderedListChildren?: LeafletBlock;
      }
    | undefined;
  const content = translateListItemContent(item?.content, warnings);
  if (!content) return null;
  const out: PlaceStreamBioBlocksOrderedList.ListItem = {
    $type: "place.stream.bio.blocks.orderedList#listItem",
    content,
  };
  if (Array.isArray(item?.children)) {
    out.children = item.children
      .map((c) => translateOrderedListItem(c, warnings))
      .filter((x): x is PlaceStreamBioBlocksOrderedList.ListItem => x !== null);
  }
  if (
    item?.unorderedListChildren?.$type === "pub.leaflet.blocks.unorderedList"
  ) {
    out.unorderedListChildren = translateUnorderedList(
      item.unorderedListChildren,
      warnings,
    );
  }
  return out;
}

function translateUnorderedListItem(
  raw: unknown,
  warnings: string[],
): PlaceStreamBioBlocksUnorderedList.ListItem | null {
  const item = raw as
    | {
        content?: LeafletBlock;
        checked?: boolean;
        children?: unknown[];
        orderedListChildren?: LeafletBlock;
      }
    | undefined;
  const content = translateListItemContent(item?.content, warnings);
  if (!content) return null;
  const out: PlaceStreamBioBlocksUnorderedList.ListItem = {
    $type: "place.stream.bio.blocks.unorderedList#listItem",
    content,
  };
  if (typeof item?.checked === "boolean") out.checked = item.checked;
  if (Array.isArray(item?.children)) {
    out.children = item.children
      .map((c) => translateUnorderedListItem(c, warnings))
      .filter(
        (x): x is PlaceStreamBioBlocksUnorderedList.ListItem => x !== null,
      );
  }
  if (item?.orderedListChildren?.$type === "pub.leaflet.blocks.orderedList") {
    out.orderedListChildren = translateOrderedList(
      item.orderedListChildren,
      warnings,
    );
  }
  return out;
}

function translateListItemContent(
  block: LeafletBlock | undefined,
  warnings: string[],
): any {
  if (!block) return null;
  switch (block.$type) {
    case "pub.leaflet.blocks.text":
      return translateText(block);
    case "pub.leaflet.blocks.header":
      return translateHeader(block);
    case "pub.leaflet.blocks.image": {
      const img = translateImage(block);
      return img ?? null;
    }
    default:
      warnings.push(
        `Skipped list item with unsupported content: ${block.$type}.`,
      );
      return null;
  }
}

function makeDivider(): PlaceStreamBioBlocksDivider.Main {
  return { $type: "place.stream.bio.blocks.divider" };
}

function makeDividerEntry(): PlaceStreamBioLayoutsPanels.BlockEntry {
  return {
    $type: "place.stream.bio.layouts.panels#blockEntry",
    block: makeDivider() as PlaceStreamBioLayoutsPanels.BlockEntry["block"],
  };
}

const LEAFLET_FACET_MAP: Record<string, string> = {
  "pub.leaflet.richtext.facet#code": "place.stream.bio.richtextFacet#code",
  "pub.leaflet.richtext.facet#bold": "place.stream.bio.richtextFacet#bold",
  "pub.leaflet.richtext.facet#italic": "place.stream.bio.richtextFacet#italic",
  "pub.leaflet.richtext.facet#highlight":
    "place.stream.bio.richtextFacet#highlight",
  "pub.leaflet.richtext.facet#underline":
    "place.stream.bio.richtextFacet#underline",
  "pub.leaflet.richtext.facet#strikethrough":
    "place.stream.bio.richtextFacet#strikethrough",
  "pub.leaflet.richtext.facet#link": "place.stream.bio.richtextFacet#link",
  "pub.leaflet.richtext.facet#id": "place.stream.bio.richtextFacet#id",
  "pub.leaflet.richtext.facet#atMention":
    "place.stream.bio.richtextFacet#atMention",
  "pub.leaflet.richtext.facet#didMention":
    "place.stream.bio.richtextFacet#didMention",
};

function passthroughFacets(raw: unknown) {
  if (!Array.isArray(raw) || raw.length === 0) return undefined;
  const facets = raw as Array<{
    index?: unknown;
    features?: Array<{ $type?: string; [k: string]: unknown }>;
    [k: string]: unknown;
  }>;
  return facets.map((facet) => {
    const features = Array.isArray(facet.features)
      ? facet.features.map((f) => {
          if (f.$type && LEAFLET_FACET_MAP[f.$type]) {
            return { ...f, $type: LEAFLET_FACET_MAP[f.$type] };
          }
          return f;
        })
      : facet.features;
    return { ...facet, features };
  }) as PlaceStreamBioRichtextFacet.Main[];
}

function stringOr(value: unknown, fallback: string): string {
  return typeof value === "string" ? value : fallback;
}
