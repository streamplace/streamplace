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
 * 0 across the fade window, and 0 once it is gone.
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

/**
 * True once a message has faded out entirely, meaning the live view drops it.
 * Scrolled back into history the message is still rendered, at full opacity.
 */
export function isChatMessageGone(
  message: ChatExpirySubject,
  now: number = Date.now(),
): boolean {
  const start = ageStart(message);
  if (!Number.isFinite(start)) return false;
  return now - start >= CHAT_MESSAGE_GONE_MS;
}

/** What a chat should render, and whether that is history rather than the live view. */
export interface ChatView<T> {
  /** Oldest first, in the order the store keeps them. */
  messages: T[];
  /** True when these are history: the whole retained list, at full strength. */
  history: boolean;
}

/**
 * The messages a chat renders, given whether the viewer is reading history.
 *
 * Reading history means the whole retained list at full strength -- the age
 * filter is a live-view concern. Nothing live left reads the same way: an
 * empty list cannot be scrolled, so filtering it would leave the viewer no way
 * in to the history the store is still holding.
 */
export function liveChatView<T extends ChatExpirySubject>(
  messages: T[],
  now: number,
  readingHistory: boolean,
): ChatView<T> {
  const live = messages.filter((message) => !isChatMessageGone(message, now));
  if (readingHistory || live.length === 0) {
    return { messages, history: true };
  }
  return { messages: live, history: false };
}
