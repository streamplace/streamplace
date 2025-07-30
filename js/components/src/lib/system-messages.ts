import { ChatMessageViewHydrated } from "streamplace";

export enum SystemMessageType {
  stream_start = "stream_start",
  stream_end = "stream_end",
  notification = "notification",
}

export interface SystemMessageMetadata {
  username?: string;
  action?: string;
  count?: number;
  duration?: string;
  reason?: string;
  streamerName?: string;
}

/**
 * Creates a system message with the proper structure
 * @param type The type of system message
 * @param text The message text
 * @param metadata Optional metadata for the message
 * @returns A properly formatted ChatMessageViewHydrated object
 */
export const createSystemMessage = (
  type: SystemMessageType,
  text: string,
  metadata?: SystemMessageMetadata,
  date: Date = new Date(),
): ChatMessageViewHydrated => {
  const now = date;

  return {
    uri: `at://did:sys:system/place.stream.chat.message/${now.getTime()}`,
    cid: `system-${now.getTime()}`,
    author: {
      did: "did:sys:system",
      handle: type, // Use handle to specify the type of system message
    },
    record: {
      text,
      createdAt: now.toISOString(),
      streamer: "system",
      $type: "place.stream.chat.message",
    },
    indexedAt: now.toISOString(),
    chatProfile: {
      color: { red: 128, green: 128, blue: 128 }, // Gray color for system messages
    },
  };
};

/**
 * System message factory functions for common scenarios
 */
export const SystemMessages = {
  streamStart: (streamerName: string): ChatMessageViewHydrated =>
    createSystemMessage(
      SystemMessageType.stream_start,
      `Now streaming - ${streamerName}`,
      {
        streamerName,
      },
    ),

  // technically, streams can't 'end' on Streamplace
  // possibly we could use deleting or editing streams (`endedAt` param) for this?
  streamEnd: (duration?: string): ChatMessageViewHydrated =>
    createSystemMessage(
      SystemMessageType.stream_end,
      duration ? `Stream has ended. Duration: ${duration}` : "Stream has ended",
      { duration },
    ),

  notification: (message: string): ChatMessageViewHydrated =>
    createSystemMessage(SystemMessageType.notification, message),
};

/**
 * Checks if a message is a system message
 * @param message The message to check
 * @returns True if the message is a system message
 */
export const isSystemMessage = (message: ChatMessageViewHydrated): boolean => {
  return message.author.did === "did:sys:system";
};

/**
 * Gets the system message type from a message
 * @param message The message to check
 * @returns The system message type or null if not a system message
 */
export const getSystemMessageType = (
  message: ChatMessageViewHydrated,
): SystemMessageType | null => {
  if (!isSystemMessage(message)) {
    return null;
  }
  return message.author.handle as SystemMessageType;
};

/**
 * Parses metadata from a system message based on its type
 * @param message The system message to parse
 * @returns The parsed metadata
 */
export const parseSystemMessageMetadata = (
  message: ChatMessageViewHydrated,
): SystemMessageMetadata => {
  const metadata: SystemMessageMetadata = {};
  const type = getSystemMessageType(message);
  const text = message.record.text;

  if (!type) return metadata;

  switch (type) {
    case "stream_end": {
      const durationMatch = text.match(/Duration:\s*(\d+:\d+(?::\d+)?)/);
      if (durationMatch) {
        metadata.duration = durationMatch[1];
      }
      break;
    }

    case "stream_start": {
      const streamerMatch = text.match(/^(.+?)\s+is now live!/);
      if (streamerMatch) {
        metadata.streamerName = streamerMatch[1];
      }
      break;
    }
  }

  return metadata;
};
