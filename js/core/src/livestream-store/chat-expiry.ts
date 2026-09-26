// Age-based expiry for live chat.
//
// Chat is a conversation, not an archive. A message holds full strength for
// the first hour, fades over the next hour, and is gone from the live view
// once it has faded out. Nothing is discarded client-side: a viewer who
// scrolls back up to read history sees expired messages again, and it is the
// node's own --chat-message-retention window that eventually stops handing
// them out at all.
//
// These helpers are pure so the React Native chat and the web chat share one
// definition of "old", and so the arithmetic can be tested without a renderer.

/** Age at which a message starts fading out of the live view. */
export const CHAT_MESSAGE_FADE_START_MS = 60 * 60 * 1000; // 1 hour

/** How long the fade lasts; a message is gone at START + DURATION. */
export const CHAT_MESSAGE_FADE_DURATION_MS = 60 * 60 * 1000; // 1 hour

/** Age at which a message has fully faded and leaves the live view. */
export const CHAT_MESSAGE_GONE_MS =
  CHAT_MESSAGE_FADE_START_MS + CHAT_MESSAGE_FADE_DURATION_MS;

export type ChatMessageTimestamp = string | number | Date;

// Epoch milliseconds, or NaN for a timestamp we cannot read. Callers treat a
// bad timestamp as "no age", so a malformed record never hides real chat.
const messageTime = (createdAt: ChatMessageTimestamp): number =>
  createdAt instanceof Date
    ? createdAt.getTime()
    : typeof createdAt === "number"
      ? createdAt
      : new Date(createdAt).getTime();

/**
 * Opacity for a message in the live view: 1 while fresh, ramping linearly to
 * 0 across the fade window, and 0 once it is gone.
 */
export function chatMessageOpacity(
  createdAt: ChatMessageTimestamp,
  now: number = Date.now(),
): number {
  const created = messageTime(createdAt);
  if (!Number.isFinite(created)) return 1;
  const age = now - created;
  if (age <= CHAT_MESSAGE_FADE_START_MS) return 1;
  if (age >= CHAT_MESSAGE_GONE_MS) return 0;
  return 1 - (age - CHAT_MESSAGE_FADE_START_MS) / CHAT_MESSAGE_FADE_DURATION_MS;
}

/**
 * True once a message has faded out entirely, meaning the live view drops it.
 * Scrolled back into history the message is still rendered, at full opacity.
 */
export function isChatMessageGone(
  createdAt: ChatMessageTimestamp,
  now: number = Date.now(),
): boolean {
  const created = messageTime(createdAt);
  if (!Number.isFinite(created)) return false;
  return now - created >= CHAT_MESSAGE_GONE_MS;
}
