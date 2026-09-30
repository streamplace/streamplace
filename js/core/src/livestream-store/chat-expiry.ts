// Age-based expiry for live chat.
//
// Chat is a conversation, not an archive. A message holds full strength for
// the first hour, fades over the next hour, and is then gone from the live
// view. Nothing is dropped from the list, though: expired messages stay in it,
// invisible, which is what keeps the history scrollable. A viewer who scrolls
// back up is reading history and sees everything the store still holds, at
// full strength. It is the node's own --chat-message-retention window that
// eventually stops handing those messages out at all.
//
// These helpers are pure so the React Native chat and the web chat share one
// definition of "old", and so the arithmetic can be tested without a renderer.

/** Age at which a message starts fading out of the live view. */
export const CHAT_MESSAGE_FADE_START_MS = 60 * 60 * 1000; // 1 hour

/** How long the fade lasts; a message is gone at START + DURATION. */
export const CHAT_MESSAGE_FADE_DURATION_MS = 60 * 60 * 1000; // 1 hour

/** Age at which a message has fully faded and is gone from the live view. */
export const CHAT_MESSAGE_GONE_MS =
  CHAT_MESSAGE_FADE_START_MS + CHAT_MESSAGE_FADE_DURATION_MS;

export type ChatMessageTimestamp = string | number | Date;

/** The parts of a message the fade reads: what it claims, and what the node took. */
export interface ChatExpirySubject {
  record?: { createdAt: ChatMessageTimestamp };
  indexedAt?: ChatMessageTimestamp | null;
}

// Epoch milliseconds, or NaN for a timestamp we cannot read. Callers treat a
// bad timestamp as "no age", so a malformed record never hides real chat.
const messageTime = (at: ChatMessageTimestamp): number =>
  at instanceof Date
    ? at.getTime()
    : typeof at === "number"
      ? at
      : new Date(at).getTime();

// When a message's age is measured from, or NaN if neither stamp is readable.
//
// A client-declared createdAt is trusted only backwards, exactly like the
// node's own indexing rule: a message dated in the future is treated as
// arriving when the node took it, so post-dating a record cannot pin it to
// the live view forever. indexedAt is that arrival time; a message with no
// readable createdAt (or none at all) falls back to it.
const ageStart = (message: ChatExpirySubject): number => {
  const created = messageTime(message.record?.createdAt ?? NaN);
  if (message.indexedAt == null) return created;
  const indexed = messageTime(message.indexedAt);
  if (!Number.isFinite(created)) return indexed;
  if (!Number.isFinite(indexed)) return created;
  return Math.min(created, indexed);
};

/**
 * Opacity for a message in the live view: 1 while fresh, ramping linearly to
 * 0 across the fade window, and 0 once it is gone. Scrolled back into history
 * the message is shown at full strength, so callers pass 1 instead.
 *
 * A message that reads 0 is gone, not absent: it stays in the list, which is
 * what keeps the list scrollable far enough back for the viewer to reach it.
 */
export function chatMessageOpacity(
  message: ChatExpirySubject,
  now: number = Date.now(),
): number {
  const start = ageStart(message);
  if (!Number.isFinite(start)) return 1;
  const age = now - start;
  if (age <= CHAT_MESSAGE_FADE_START_MS) return 1;
  if (age >= CHAT_MESSAGE_GONE_MS) return 0;
  return 1 - (age - CHAT_MESSAGE_FADE_START_MS) / CHAT_MESSAGE_FADE_DURATION_MS;
}
