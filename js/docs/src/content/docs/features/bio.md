---
title: Bio Page
description: Create a customizable bio page for your Streamplace channel.
---

Your Streamplace bio is a full-featured page that lives on your channel. It includes a long-form description, pinned social links, and a customizable body made of content blocks.

## Description

The bio description is long-form rich text that appears in your bio header, and it's distinct from your Bluesky profile description.

## Social links

You can pin up to 16 social or platform links in your bio header. These appear alongside your description and let viewers find you elsewhere.

## Layout

The body of your bio uses a **panels layout**. Panels are columns of content blocks that arrange themselves into a masonry grid: each panel gets placed into the shortest column, so content packs tightly without awkward gaps. Columns center within the page and the number of columns adapts to the available width. On mobile, panels stack vertically.

You can have up to 8 panels, each containing any mix of these blocks:

- Text, Header, Image
- Ordered List, Unordered List
- Blockquote, Divider
- Link, Social Links
- Livestream, Schedule
- Bluesky Post, Embed

Each block can be aligned left, center, or right within its panel.

## Tips

- Order matters: panels are placed into the grid in sequence, so put important content first.
- Mix block types within a panel for visual variety.
- Use dividers to separate sections inside a panel.

# Importing from Leaflet

If you have documents on [Leaflet](https://leaflet.pub), you can import them directly into your bio. In Settings > Bio, paste either a Leaflet URL or AT-URI:

```
https://leaflet.pub/p/did:plc:.../abc123
at://did:plc:.../site.standard.document/abc123
```

The import must be your own document. Two import modes are available:

- Full import: All pages from the Leaflet document are combined into a single-panel bio. Pages are separated by divider blocks. Text blocks, images, embeds, and most other block types translate directly. Some Leaflet-specific blocks (code, math, polls) are skipped with a warning.

- Select Panels: Opens a range selector that lets you carve the document into multiple panels. Each divider or horizontal rule in the Leaflet doc becomes a natural split point. You can adjust range boundaries to group blocks however you want. Each range becomes its own panel in the masonry layout.

After importing, your bio stores the source reference. Reopening the Leaflet import in Settings will automatically reload the document and pre-fill the range selector, so you can re-import with different panel splits anytime.

## Designing assets

Each panel renders as a fixed 350px-wide column, so size your content accordingly.

### Images

Images fill the full column width and use the aspect ratio stored in the block metadata. If no aspect ratio is set, images default to 16:9. Leaflet should set these automatically, but if you don't use Leaflet please keep that in mind.

- **Recommended resolution**: At least 700px wide for crisp rendering on high-DPI screens. The column is 350px, but 2x gives you headroom.
- **Alt text**: Please provide alt text! It benefits everyone, not just only those who rely on it for accessibility!

### Layouting behavior

The masonry grid places each panel into whichever column is currently shortest. This means:

- A panel with a tall image or long text won't create a gap below a shorter neighboring panel.
- Panels flow left to right, then wrap. The panel order you set in the editor is the order they're placed.
- On wide screens (1200px+) you'll see 3 columns. On narrow screens (under 700px), everything stacks to a single column.
- Panels center within the available space — leftover horizontal room is split evenly on both sides.
