const DESKTOP_CHAT_OPEN_KEY = "streamplace:chat-open";

export function chatPreferenceKey(isWide: boolean): string | null {
  return isWide ? DESKTOP_CHAT_OPEN_KEY : null;
}

export function shouldOpenChat(
  isOffline: boolean,
  savedPreference: string | null,
): boolean {
  if (isOffline) return false;
  return savedPreference !== "false";
}

export function chatOpenAfterLivenessChange({
  isOffline,
  wasOffline,
  userChangedChat,
  preferredChatOpen,
  currentChatOpen,
}: {
  isOffline: boolean;
  wasOffline: boolean;
  userChangedChat: boolean;
  preferredChatOpen: boolean;
  currentChatOpen: boolean;
}): boolean {
  if (isOffline && !wasOffline) return false;
  if (!isOffline && wasOffline && !userChangedChat) return preferredChatOpen;
  return currentChatOpen;
}
