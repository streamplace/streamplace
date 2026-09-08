/**
 * Column widths shared by the card stream layout and the social shell that
 * sizes its content column around it. The design's numbers are for a 1440px
 * window (600 feed, 407 chat); on wider windows the chat column grows so it
 * doesn't sit in a sea of margin.
 */
export const FEED_WIDTH = 600;
export const CHAT_WIDTH_MIN = 407;
export const CHAT_WIDTH_MAX = 560;
const DESIGN_WINDOW = 1440;

/** Chat column width for a window of the given width. */
export function chatWidthFor(windowWidth: number): number {
  const extra = Math.max(0, windowWidth - DESIGN_WINDOW) * 0.4;
  return Math.round(
    Math.min(CHAT_WIDTH_MAX, Math.max(CHAT_WIDTH_MIN, CHAT_WIDTH_MIN + extra)),
  );
}

/** Feed + hairline + chat: the stream page's content column. */
export function streamColumnWidthFor(windowWidth: number): number {
  return FEED_WIDTH + 1 + chatWidthFor(windowWidth);
}

/** The lone feed column (plus its right hairline) on every other page. */
export const FEED_COLUMN_WIDTH = FEED_WIDTH + 1;
