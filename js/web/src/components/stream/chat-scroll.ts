type ChatScrollElement = Pick<
  HTMLElement,
  "clientHeight" | "scrollHeight" | "scrollTop"
>;

export function initializeChatScroll(
  element: ChatScrollElement,
  reversed: boolean,
): boolean {
  if (element.clientHeight === 0) return false;
  element.scrollTop = reversed ? 0 : element.scrollHeight;
  return true;
}
